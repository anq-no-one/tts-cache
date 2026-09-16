import Foundation

public enum AudioSource: String, Sendable, Equatable, Codable {
    case memory
    case disk
    case proxy
    case direct
    case system
}

public struct ProxyFailure: Sendable, Equatable {
    public let host: String
    public let statusCode: Int?
    public let latencyMs: Double

    public init(host: String, statusCode: Int?, latencyMs: Double) {
        self.host = host
        self.statusCode = statusCode
        self.latencyMs = latencyMs
    }
}

public struct SentenceStatus: Sendable, Equatable, Decodable {
    public let index: Int
    public let key: String
    public let status: String
    public let error: String?
}

public struct SynthResult: Sendable, Equatable {
    public let audio: Data
    public let source: AudioSource
    public let failure: ProxyFailure?
    public let sentenceStatuses: [SentenceStatus]
    public let region: String?

    public init(audio: Data, source: AudioSource, failure: ProxyFailure?, sentenceStatuses: [SentenceStatus], region: String? = nil) {
        self.audio = audio
        self.source = source
        self.failure = failure
        self.sentenceStatuses = sentenceStatuses
        self.region = region
    }
}

public struct TTSClient: Sendable {
    public let endpoints: [URL]
    public let appToken: String
    public let session: URLSession
    public let healthTimeout: TimeInterval
    public let requestTimeout: TimeInterval
    public let directFetch: (@Sendable (String) async throws -> Data)?

    public init(
        endpoints: [URL],
        appToken: String,
        session: URLSession = .shared,
        healthTimeout: TimeInterval = 2,
        requestTimeout: TimeInterval = 30,
        directFetch: (@Sendable (String) async throws -> Data)? = nil
    ) {
        self.endpoints = endpoints
        self.appToken = appToken
        self.session = session
        self.healthTimeout = healthTimeout
        self.requestTimeout = requestTimeout
        self.directFetch = directFetch
    }

    public static func orderByLatency(endpoints: [URL], latencies: [URL: Double]) -> [URL] {
        endpoints.enumerated().sorted { a, b in
            let la = latencies[a.element] ?? .infinity
            let lb = latencies[b.element] ?? .infinity
            if la != lb {
                return la < lb
            }
            return a.offset < b.offset
        }.map(\.element)
    }

    public func synthesize(text: String, config: TTSCacheConfig) async -> SynthResult {
        let latencies = await probeLatencies()
        let ranked = Self.orderByLatency(endpoints: endpoints, latencies: latencies)
        var lastFailure: ProxyFailure?
        for endpoint in ranked {
            let started = Date()
            do {
                let (data, response) = try await post(text: text, config: config, endpoint: endpoint)
                let elapsedMs = Date().timeIntervalSince(started) * 1000
                guard let http = response as? HTTPURLResponse else {
                    lastFailure = ProxyFailure(host: Self.host(of: endpoint), statusCode: nil, latencyMs: elapsedMs)
                    continue
                }
                switch http.statusCode {
                case 200..<300:
                    return SynthResult(
                        audio: data,
                        source: .proxy,
                        failure: nil,
                        sentenceStatuses: Self.sentenceStatuses(from: http),
                        region: Self.region(from: http)
                    )
                case 429:
                    lastFailure = ProxyFailure(host: Self.host(of: endpoint), statusCode: http.statusCode, latencyMs: elapsedMs)
                case 400..<500:
                    return SynthResult(
                        audio: Data(),
                        source: .proxy,
                        failure: ProxyFailure(host: Self.host(of: endpoint), statusCode: http.statusCode, latencyMs: elapsedMs),
                        sentenceStatuses: Self.sentenceStatuses(from: http),
                        region: Self.region(from: http)
                    )
                default:
                    lastFailure = ProxyFailure(host: Self.host(of: endpoint), statusCode: http.statusCode, latencyMs: elapsedMs)
                }
            } catch {
                let elapsedMs = Date().timeIntervalSince(started) * 1000
                lastFailure = ProxyFailure(host: Self.host(of: endpoint), statusCode: nil, latencyMs: elapsedMs)
            }
        }
        if let directFetch {
            do {
                let audio = try await directFetch(text)
                return SynthResult(audio: audio, source: .direct, failure: lastFailure, sentenceStatuses: [])
            } catch {
                return SynthResult(audio: Data(), source: .system, failure: lastFailure, sentenceStatuses: [])
            }
        }
        return SynthResult(audio: Data(), source: .system, failure: lastFailure, sentenceStatuses: [])
    }

    static func region(from response: HTTPURLResponse) -> String? {
        guard let raw = response.value(forHTTPHeaderField: "X-Region"), !raw.isEmpty else {
            return nil
        }
        return raw
    }

    static func sentenceStatuses(from response: HTTPURLResponse) -> [SentenceStatus] {
        guard let raw = response.value(forHTTPHeaderField: "X-Sentence-Statuses"),
            let data = raw.data(using: .utf8)
        else {
            return []
        }
        return (try? JSONDecoder().decode([SentenceStatus].self, from: data)) ?? []
    }

    static func host(of endpoint: URL) -> String {
        endpoint.host ?? endpoint.absoluteString
    }

    private func post(text: String, config: TTSCacheConfig, endpoint: URL) async throws -> (Data, URLResponse) {
        var request = TTSCache.synthesizeRequest(text: text, config: config)
        request.url = endpoint.appendingPathComponent("/v1/synthesize")
        request.timeoutInterval = requestTimeout
        request.setValue("Bearer \(appToken)", forHTTPHeaderField: "Authorization")
        return try await session.data(for: request)
    }

    private func probeLatencies() async -> [URL: Double] {
        await withTaskGroup(of: (URL, Double).self) { group in
            for endpoint in endpoints {
                group.addTask {
                    let started = Date()
                    var request = URLRequest(url: endpoint.appendingPathComponent("/healthz"))
                    request.httpMethod = "GET"
                    request.timeoutInterval = self.healthTimeout
                    do {
                        let (_, response) = try await self.session.data(for: request)
                        guard let http = response as? HTTPURLResponse, (200..<300).contains(http.statusCode) else {
                            return (endpoint, .infinity)
                        }
                        let ms = Date().timeIntervalSince(started) * 1000
                        return (endpoint, ms.rounded())
                    } catch {
                        return (endpoint, .infinity)
                    }
                }
            }
            var out: [URL: Double] = [:]
            for await (url, ms) in group {
                out[url] = ms
            }
            return out
        }
    }
}
