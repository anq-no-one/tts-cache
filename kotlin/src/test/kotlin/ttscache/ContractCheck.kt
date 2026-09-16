package ttscache

import java.io.File

private data class Vector(val input: String, val normalized: String)

private fun readJsonString(json: String, start: Int): Pair<String, Int> {
    var i = start
    check(i < json.length && json[i] == '"') { "expected string at offset $i" }
    i++
    val sb = StringBuilder()
    while (true) {
        check(i < json.length) { "unterminated string" }
        val c = json[i++]
        if (c == '"') return sb.toString() to i
        if (c != '\\') {
            sb.append(c)
            continue
        }
        check(i < json.length) { "bad escape at end of input" }
        when (val e = json[i++]) {
            '"', '\\', '/' -> sb.append(e)
            'n' -> sb.append('\n')
            'r' -> sb.append('\r')
            't' -> sb.append('\t')
            'b' -> sb.append('\b')
            'u' -> {
                check(i + 4 <= json.length) { "bad unicode escape" }
                sb.append(json.substring(i, i + 4).toInt(16).toChar())
                i += 4
            }
            else -> error("unsupported escape \\$e")
        }
    }
}

private fun parseVectors(json: String): List<Vector> {
    var i = 0
    fun skipWs() {
        while (i < json.length && json[i].isWhitespace()) i++
    }
    fun expect(ch: Char) {
        skipWs()
        check(i < json.length && json[i] == ch) { "expected '$ch' at offset $i" }
        i++
    }
    expect('[')
    val out = mutableListOf<Vector>()
    skipWs()
    if (i < json.length && json[i] == ']') return out
    while (true) {
        expect('{')
        var input: String? = null
        var normalized: String? = null
        while (true) {
            skipWs()
            val (key, next) = readJsonString(json, i)
            i = next
            expect(':')
            skipWs()
            val (value, after) = readJsonString(json, i)
            i = after
            when (key) {
                "input" -> input = value
                "normalized" -> normalized = value
            }
            skipWs()
            check(i < json.length) { "truncated vectors file" }
            if (json[i] == ',') {
                i++
                continue
            }
            check(json[i] == '}') { "expected '}' at offset $i" }
            i++
            break
        }
        check(input != null && normalized != null) { "vector entry missing input/normalized" }
        out.add(Vector(input, normalized))
        skipWs()
        check(i < json.length) { "truncated vectors file" }
        if (json[i] == ',') {
            i++
            continue
        }
        check(json[i] == ']') { "expected ']' at offset $i" }
        i++
        break
    }
    return out
}

fun main() {
    val candidates = listOf(
        File("../vectors/vectors.json"),
        File("vectors/vectors.json"),
        File("../../vectors/vectors.json")
    )
    val vectorsFile = candidates.firstOrNull { it.isFile }
        ?: error("vectors.json not found, tried: " + candidates.joinToString(", "))
    println("vectors file: ${vectorsFile.path}")

    val vectors = parseVectors(vectorsFile.readText(Charsets.UTF_8))
    check(vectors.isNotEmpty()) { "no vectors loaded" }
    vectors.forEachIndexed { index, v ->
        val got = TtsCache.normalize(v.input)
        check(got == v.normalized) { "vector $index mismatch: got \"$got\" want \"${v.normalized}\"" }
    }
    println("vectors: ${vectors.size}/${vectors.size} normalization checks passed")

    check(TtsCache.speedKey(0.0) == "1") { "speedKey(0) must be \"1\"" }
    check(TtsCache.speedKey(1.08) == "1.080") { "speedKey(1.08) must be \"1.080\", got \"${TtsCache.speedKey(1.08)}\"" }
    println("speedKey: 0 -> 1, 1.08 -> 1.080 ok")

    val goldenConfig = TtsCacheConfig(
        baseUrl = "http://localhost:8080",
        voiceId = "v1",
        modelId = "fish-audio/s2.1-pro",
        speed = 1.08,
        format = "mp3_44100_128",
        language = "en"
    )
    val golden = TtsCache.cacheKey("Rest for 30 seconds.", goldenConfig)
    check(golden == "v1-feab4a17bc7070e8") { "golden key mismatch: got $golden" }
    println("golden key: $golden ok")

    val noisy = TtsCache.cacheKey("  Rest   for 30 seconds . ", goldenConfig)
    val clean = TtsCache.cacheKey("rest for 30 seconds.", goldenConfig)
    check(noisy == clean) { "key must ignore case and spacing noise" }
    println("key ignores case/spacing noise ok")

    val base = TtsCacheConfig(baseUrl = "http://localhost:8080", voiceId = "voice1")
    val baseKey = TtsCache.cacheKey("Rest for 30 seconds.", base)
    check(TtsCache.cacheKey("Rest for 30 seconds.", base.copy(speed = 1.5)) != baseKey) {
        "key must change with speed"
    }
    check(TtsCache.cacheKey("Rest for 30 seconds.", base.copy(modelId = "other-model")) != baseKey) {
        "key must change with model"
    }
    check(TtsCache.cacheKey("Different sentence.", base) != baseKey) {
        "key must change with text"
    }
    println("key changes on sound fields ok")

    val parts = TtsCache.splitSentences("Rest for 30 seconds. Next up is push ups.")
    check(parts.size == 2) { "expected 2 sentences, got $parts" }
    check(parts[0] == "Rest for 30 seconds.") { "first sentence wrong: ${parts[0]}" }
    check(TtsCache.splitSentences("no punctuation here") == listOf("no punctuation here")) {
        "sentence without terminator must come back whole"
    }
    check(TtsCache.splitSentences("   ").isEmpty()) { "blank text must give no sentences" }
    println("splitSentences: order and edge cases ok")

    val req = TtsCache.synthesizeRequest("Rest for 30 seconds.", base)
    check(req.url == "http://localhost:8080/v1/synthesize") { "request url wrong: ${req.url}" }
    check(req.jsonBody.contains("\"voice_id\":\"voice1\"")) { "request body missing voice_id: ${req.jsonBody}" }
    check(req.jsonBody.contains("\"text\":\"Rest for 30 seconds.\"")) { "request body missing text" }
    println("synthesizeRequest: url and body ok")

    println("ALL CONTRACT CHECKS PASSED")
}
