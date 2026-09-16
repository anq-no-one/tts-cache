import Foundation
import Testing

@testable import TTSCache

final class StubURLProtocol: URLProtocol {
    static var handler: ((URLRequest) throws -> (HTTPURLResponse, Data))?
    static var seen: [URLRequest] = []
    static let lock = NSLock()

    override class func canInit(with request: URLRequest) -> Bool { true }

    override class func canonicalRequest(for request: URLRequest) -> URLRequest { request }

    override func startLoading() {
        Self.lock.lock()
        Self.seen.append(request)
        Self.lock.unlock()
        do {
            let (response, data) = try Self.handler!(request)
            client?.urlProtocol(self, didReceive: response, cacheStoragePolicy: .notAllowed)
            client?.urlProtocol(self, didLoad: data)
            client?.urlProtocolDidFinishLoading(self)
        } catch {
            client?.urlProtocol(self, didFailWithError: error)
        }
    }

    override func stopLoading() {}

    static func reset() {
        lock.lock()
        seen = []
        handler = nil
        lock.unlock()
    }

    static func posts() -> [URLRequest] {
        lock.lock()
        defer { lock.unlock() }
        return seen.filter { $0.url?.path == "/v1/synthesize" }
    }
}

func makeStubSession() -> URLSession {
    let config = URLSessionConfiguration.ephemeral
    config.protocolClasses = [StubURLProtocol.self]
    return URLSession(configuration: config)
}

func okResponse(for request: URLRequest, status: Int = 200, headers: [String: String]? = nil, body: Data = Data("ok".utf8)) -> (HTTPURLResponse, Data) {
    (HTTPURLResponse(url: request.url!, statusCode: status, httpVersion: nil, headerFields: headers)!, body)
}

@Suite(.serialized)
struct TTSClientTests {
    let a = URL(string: "https://a.example")!
    let b = URL(string: "https://b.example")!

    @Test func latencyOrderingIsPure() {
        let c = URL(string: "https://c.example")!
        let ranked = TTSClient.orderByLatency(endpoints: [a, b, c], latencies: [a: 90, b: 10])
        #expect(ranked == [b, a, c])
    }

    @Test func failoverTriesEndpointsInLatencyOrder() async {
        StubURLProtocol.reset()
        StubURLProtocol.handler = { request in
            if request.url?.path == "/healthz" {
                if request.url?.host == "b.example" {
                    Thread.sleep(forTimeInterval: 0.1)
                }
                return okResponse(for: request)
            }
            #expect(request.value(forHTTPHeaderField: "Authorization") == "Bearer test-token")
            if request.url?.host == "a.example" {
                return okResponse(for: request, status: 502, body: Data("bad".utf8))
            }
            return okResponse(for: request, body: Data("audio-b".utf8))
        }
        let client = TTSClient(endpoints: [a, b], appToken: "test-token", session: makeStubSession())
        let config = TTSCacheConfig(baseURL: a, voiceID: "v1")
        let result = await client.synthesize(text: "Rest for 30 seconds.", config: config)
        #expect(result.source == .proxy)
        #expect(result.audio == Data("audio-b".utf8))
        #expect(result.failure == nil)
        #expect(StubURLProtocol.posts().map { $0.url?.host } == ["a.example", "b.example"])
    }

    @Test func unhealthyEndpointSortsLast() async {
        StubURLProtocol.reset()
        StubURLProtocol.handler = { request in
            if request.url?.path == "/healthz" {
                let status = request.url?.host == "a.example" ? 500 : 200
                return okResponse(for: request, status: status)
            }
            return okResponse(for: request, body: Data("audio-b".utf8))
        }
        let client = TTSClient(endpoints: [a, b], appToken: "test-token", session: makeStubSession())
        let config = TTSCacheConfig(baseURL: a, voiceID: "v1")
        let result = await client.synthesize(text: "Hello.", config: config)
        #expect(result.source == .proxy)
        #expect(result.audio == Data("audio-b".utf8))
        #expect(StubURLProtocol.posts().map { $0.url?.host } == ["b.example"])
    }

    @Test func partialContentParsesStatuses() async {
        StubURLProtocol.reset()
        let statuses = #"[{"index":0,"key":"v1-abc","status":"hit"},{"index":1,"key":"v1-def","status":"error","error":"boom"}]"#
        StubURLProtocol.handler = { request in
            if request.url?.path == "/healthz" {
                return okResponse(for: request)
            }
            return okResponse(
                for: request,
                status: 206,
                headers: [
                    "Content-Type": "audio/mpeg",
                    "X-Sentence-Count": "2",
                    "X-Sentence-Statuses": statuses,
                ],
                body: Data("part".utf8)
            )
        }
        let client = TTSClient(endpoints: [a], appToken: "test-token", session: makeStubSession())
        let config = TTSCacheConfig(baseURL: a, voiceID: "v1")
        let result = await client.synthesize(text: "One. Two.", config: config)
        #expect(result.source == .proxy)
        #expect(result.audio == Data("part".utf8))
        #expect(result.sentenceStatuses.count == 2)
        #expect(result.sentenceStatuses[0].status == "hit")
        #expect(result.sentenceStatuses[1] == SentenceStatus(index: 1, key: "v1-def", status: "error", error: "boom"))
    }

    @Test func directFallbackWhenAllEndpointsFail() async {
        StubURLProtocol.reset()
        StubURLProtocol.handler = { request in
            if request.url?.path == "/healthz" {
                return okResponse(for: request)
            }
            return okResponse(for: request, status: 502, body: Data("down".utf8))
        }
        let client = TTSClient(
            endpoints: [a, b],
            appToken: "test-token",
            session: makeStubSession(),
            directFetch: { _ in Data("direct".utf8) }
        )
        let config = TTSCacheConfig(baseURL: a, voiceID: "v1")
        let result = await client.synthesize(text: "Hello.", config: config)
        #expect(result.source == .direct)
        #expect(result.audio == Data("direct".utf8))
        #expect(result.failure?.statusCode == 502)
    }

    @Test func systemSilenceWhenNoDirectFetch() async {
        StubURLProtocol.reset()
        StubURLProtocol.handler = { request in
            if request.url?.path == "/healthz" {
                return okResponse(for: request)
            }
            return okResponse(for: request, status: 500, body: Data("down".utf8))
        }
        let client = TTSClient(endpoints: [a], appToken: "test-token", session: makeStubSession())
        let config = TTSCacheConfig(baseURL: a, voiceID: "v1")
        let result = await client.synthesize(text: "Hello.", config: config)
        #expect(result.source == .system)
        #expect(result.audio.isEmpty)
        #expect(result.failure?.host == "a.example")
        #expect(result.failure?.statusCode == 500)
    }
}
