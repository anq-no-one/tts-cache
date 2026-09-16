import Foundation
import TTSCache

func run() async {
    let config = TTSCacheConfig(
        baseURL: URL(string: "http://localhost:8080")!,
        voiceID: "test-voice"
    )
    let client = TTSClient(
        endpoints: [URL(string: "http://localhost:8080")!],
        appToken: "app-token"
    )
    let result = await client.synthesize(text: "Hello. World.", config: config)
    switch result.source {
    case .proxy:
        print("proxy bytes=\(result.audio.count)")
    case .direct:
        print("direct bytes=\(result.audio.count)")
    case .memory, .disk, .system:
        print("\(result.source.rawValue) bytes=\(result.audio.count)")
    }
    for status in result.sentenceStatuses {
        print("\(status.index) \(status.key) \(status.status)")
    }
    if let region = result.region {
        print("region=\(region)")
    }
}
