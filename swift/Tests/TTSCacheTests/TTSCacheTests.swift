import Foundation
import Testing

@testable import TTSCache

struct Vector: Decodable {
    let input: String
    let normalized: String
}

@Test func sharedVectorsMatchServer() throws {
    let here = URL(filePath: #filePath).deletingLastPathComponent()
    let url = here
        .deletingLastPathComponent()
        .deletingLastPathComponent()
        .deletingLastPathComponent()
        .appending(path: "vectors/vectors.json")
    let data = try Data(contentsOf: url)
    let vectors = try JSONDecoder().decode([Vector].self, from: data)
    for v in vectors {
        #expect(TTSCache.normalize(v.input) == v.normalized)
    }
}

@Test func keyIgnoresCaseAndSpacing() {
    let config = TTSCacheConfig(baseURL: URL(string: "http://localhost:8080")!, voiceID: "v1")
    let a = TTSCache.cacheKey(text: "  Rest   for 30 seconds . ", config: config)
    let b = TTSCache.cacheKey(text: "rest for 30 seconds.", config: config)
    #expect(a == b)
}

@Test func keyChangesOnSoundParams() {
    let base = TTSCacheConfig(baseURL: URL(string: "http://localhost:8080")!, voiceID: "v1")
    var other = base
    other.speed = 1.5
    #expect(TTSCache.cacheKey(text: "hello.", config: base) != TTSCache.cacheKey(text: "hello.", config: other))
}

@Test func goldenKeyMatchesServer() {
    let config = TTSCacheConfig(
        baseURL: URL(string: "http://localhost:8080")!,
        voiceID: "v1",
        modelID: "fish-audio/s2.1-pro",
        speed: 1.08
    )
    #expect(TTSCache.cacheKey(text: "Rest for 30 seconds.", config: config) == "v1-feab4a17bc7070e8")
}

@Test func splitKeepsOrder() {
    let parts = TTSCache.splitSentences("Rest for 30 seconds. Next up is push ups.")
    #expect(parts.count == 2)
}

@Test func splitBlankIsEmpty() {
    #expect(TTSCache.splitSentences("").isEmpty)
    #expect(TTSCache.splitSentences("   ").isEmpty)
}

@Test func audioSourceCodableRoundTrip() throws {
    for source in [AudioSource.memory, .disk, .proxy, .direct, .system] {
        let data = try JSONEncoder().encode(source)
        #expect(try JSONDecoder().decode(AudioSource.self, from: data) == source)
    }
}
