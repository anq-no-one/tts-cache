import CryptoKit
import Foundation
import NaturalLanguage

public struct TTSCacheConfig: Sendable {
    public var baseURL: URL
    public var voiceID: String
    public var modelID: String
    public var speed: Double
    public var format: String
    public var language: String

    public init(
        baseURL: URL,
        voiceID: String,
        modelID: String = "fish-audio/s2.1-pro",
        speed: Double = 1.0,
        format: String = "mp3_44100_128",
        language: String = "en"
    ) {
        self.baseURL = baseURL
        self.voiceID = voiceID
        self.modelID = modelID
        self.speed = speed
        self.format = format
        self.language = language
    }
}

public enum TTSCache {
    public static let keyVersion = "v1"

    public static func normalize(_ text: String) -> String {
        var s = text.trimmingCharacters(in: .whitespacesAndNewlines)
        s = s.replacingOccurrences(of: "\\s+", with: " ", options: .regularExpression)
        s = s.replacingOccurrences(of: " .", with: ".")
        s = s.replacingOccurrences(of: " ,", with: ",")
        return s.lowercased()
    }

    public static func splitSentences(_ text: String) -> [String] {
        let tokenizer = NLTokenizer(unit: .sentence)
        tokenizer.string = text
        var out: [String] = []
        tokenizer.enumerateTokens(in: text.startIndex..<text.endIndex) { range, _ in
            let part = String(text[range]).trimmingCharacters(in: .whitespacesAndNewlines)
            if !part.isEmpty {
                out.append(part)
            }
            return true
        }
        if out.isEmpty {
            let trimmed = text.trimmingCharacters(in: .whitespacesAndNewlines)
            if !trimmed.isEmpty {
                out = [trimmed]
            }
        }
        return out
    }

    public static func cacheKey(text: String, config: TTSCacheConfig) -> String {
        let parts = [
            keyVersion,
            normalize(text),
            config.voiceID.trimmingCharacters(in: .whitespaces),
            config.modelID.trimmingCharacters(in: .whitespaces),
            String(format: "%.3f", config.speed),
            config.format.trimmingCharacters(in: .whitespaces),
            config.language.trimmingCharacters(in: .whitespaces),
        ]
        let digest = SHA256.hash(data: Data(parts.joined(separator: "|").utf8))
        let hex = digest.map { String(format: "%02x", $0) }.joined()
        return keyVersion + "-" + String(hex.prefix(16))
    }

    public static func synthesizeRequest(text: String, config: TTSCacheConfig) -> URLRequest {
        var request = URLRequest(url: config.baseURL.appendingPathComponent("/v1/synthesize"))
        request.httpMethod = "POST"
        request.setValue("application/json", forHTTPHeaderField: "Content-Type")
        let body: [String: Any] = [
            "text": text,
            "voice_id": config.voiceID,
            "model_id": config.modelID,
            "speed": config.speed,
            "format": config.format,
            "language": config.language,
        ]
        request.httpBody = try? JSONSerialization.data(withJSONObject: body)
        return request
    }
}
