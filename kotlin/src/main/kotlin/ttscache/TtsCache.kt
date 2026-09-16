package ttscache

import java.net.HttpURLConnection
import java.net.URL
import java.security.MessageDigest
import java.util.Locale

data class TtsCacheConfig(
    val baseUrl: String,
    val voiceId: String,
    val modelId: String = "fish-audio/s2.1-pro",
    val speed: Double = 1.0,
    val format: String = "mp3_44100_128",
    val language: String = "en"
)

data class SynthesizeRequest(
    val url: String,
    val jsonBody: String
)

data class SynthesizeResponse(
    val statusCode: Int,
    val audio: ByteArray,
    val sentenceStatuses: String,
    val region: String
)

object TtsCache {
    const val keyVersion = "v1"

    private val spaces = Regex("\\s+")
    private val sentenceEnd = Regex("[.!?]+\\s*")

    fun normalize(text: String): String {
        val collapsed = spaces.replace(text.trim(), " ")
        return collapsed.replace(" .", ".").replace(" ,", ",").lowercase()
    }

    fun speedKey(speed: Double): String {
        if (speed == 0.0) return "1"
        return String.format(Locale.US, "%.3f", speed)
    }

    fun cacheKey(text: String, config: TtsCacheConfig): String {
        val parts = listOf(
            keyVersion,
            normalize(text),
            config.voiceId.trim(),
            config.modelId.trim(),
            speedKey(config.speed),
            config.format.trim(),
            config.language.trim()
        )
        val raw = parts.joinToString("|").toByteArray(Charsets.UTF_8)
        val digest = MessageDigest.getInstance("SHA-256").digest(raw)
        val hex = digest.joinToString("") { (it.toInt() and 0xFF).toString(16).padStart(2, '0') }
        return "$keyVersion-${hex.substring(0, 16)}"
    }

    fun splitSentences(text: String): List<String> {
        val out = mutableListOf<String>()
        var start = 0
        for (match in sentenceEnd.findAll(text)) {
            val part = text.substring(start, match.range.last + 1).trim()
            if (part.isNotEmpty()) out.add(part)
            start = match.range.last + 1
        }
        val rest = text.substring(start).trim()
        if (rest.isNotEmpty()) out.add(rest)
        if (out.isEmpty() && text.trim().isNotEmpty()) out.add(text.trim())
        return out
    }

    fun synthesizeRequest(text: String, config: TtsCacheConfig): SynthesizeRequest {
        val url = config.baseUrl.trimEnd('/') + "/v1/synthesize"
        val body = buildString {
            append("{\"text\":\"")
            append(jsonEscape(text))
            append("\",\"voice_id\":\"")
            append(jsonEscape(config.voiceId))
            append("\",\"model_id\":\"")
            append(jsonEscape(config.modelId))
            append("\",\"speed\":")
            append(numberLiteral(config.speed))
            append(",\"format\":\"")
            append(jsonEscape(config.format))
            append("\",\"language\":\"")
            append(jsonEscape(config.language))
            append("\"}")
        }
        return SynthesizeRequest(url, body)
    }

    fun fetchSynthesize(
        endpointUrl: String,
        appToken: String,
        request: SynthesizeRequest,
        connectTimeoutMs: Int = 5000,
        readTimeoutMs: Int = 30000
    ): SynthesizeResponse {
        val connection = URL(endpointUrl).openConnection() as HttpURLConnection
        try {
            connection.requestMethod = "POST"
            connection.connectTimeout = connectTimeoutMs
            connection.readTimeout = readTimeoutMs
            connection.doOutput = true
            connection.setRequestProperty("Content-Type", "application/json")
            connection.setRequestProperty("Accept", "audio/mpeg")
            connection.setRequestProperty("Authorization", "Bearer $appToken")
            val body = request.jsonBody.toByteArray(Charsets.UTF_8)
            connection.outputStream.use { it.write(body) }
            val status = connection.responseCode
            val stream = if (status < 400) connection.inputStream else connection.errorStream
            val audio = stream?.readBytes() ?: ByteArray(0)
            return SynthesizeResponse(
                statusCode = status,
                audio = audio,
                sentenceStatuses = responseHeader(connection, "X-Sentence-Statuses"),
                region = responseHeader(connection, "X-Region")
            )
        } finally {
            connection.disconnect()
        }
    }

    private fun responseHeader(connection: HttpURLConnection, name: String): String {
        connection.headerFields.forEach { (key, values) ->
            if (key != null && key.equals(name, ignoreCase = true)) return values?.firstOrNull() ?: ""
        }
        return ""
    }

    private fun numberLiteral(speed: Double): String {
        if (speed == 0.0) return "0"
        return speed.toString()
    }

    private fun jsonEscape(value: String): String {
        val sb = StringBuilder()
        for (c in value) {
            when (c) {
                '"' -> sb.append("\\\"")
                '\\' -> sb.append("\\\\")
                '\n' -> sb.append("\\n")
                '\r' -> sb.append("\\r")
                '\t' -> sb.append("\\t")
                '\b' -> sb.append("\\b")
                else -> {
                    if (c < ' ') sb.append("\\u" + c.code.toString(16).padStart(4, '0'))
                    else sb.append(c)
                }
            }
        }
        return sb.toString()
    }
}
