package ttscache

import com.sun.net.httpserver.HttpServer
import java.io.File
import java.io.IOException
import java.net.InetSocketAddress
import java.net.ServerSocket

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

    checkNetworking()

    println("ALL CONTRACT CHECKS PASSED")
}

private fun checkNetworking() {
    val okAudio = byteArrayOf(1, 2, 3, 4)
    val okStatuses = """[{"index":0,"key":"v1-abc","status":"synthesized"}]"""
    val partialAudio = byteArrayOf(5, 6)
    val partialStatuses =
        """[{"index":0,"key":"v1-abc","status":"hit"},{"index":1,"key":"v1-def","status":"synthesized"}]"""
    var lastAuth = ""

    val server = HttpServer.create(InetSocketAddress("127.0.0.1", 0), 0)
    server.createContext("/v1/synthesize") { exchange ->
        lastAuth = exchange.requestHeaders.getFirst("Authorization") ?: ""
        exchange.requestBody.use { it.readBytes() }
        exchange.responseHeaders.set("Content-Type", "audio/mpeg")
        exchange.responseHeaders.set("X-Sentence-Statuses", okStatuses)
        exchange.responseHeaders.set("X-Region", "eu-west")
        exchange.sendResponseHeaders(200, okAudio.size.toLong())
        exchange.responseBody.use { it.write(okAudio) }
    }
    server.createContext("/v1/partial") { exchange ->
        exchange.requestBody.use { it.readBytes() }
        exchange.responseHeaders.set("Content-Type", "audio/mpeg")
        exchange.responseHeaders.set("X-Sentence-Statuses", partialStatuses)
        exchange.responseHeaders.set("X-Region", "eu-west")
        exchange.sendResponseHeaders(206, partialAudio.size.toLong())
        exchange.responseBody.use { it.write(partialAudio) }
    }
    server.start()
    try {
        val loopback = "http://127.0.0.1:${server.address.port}"
        val netConfig = TtsCacheConfig(baseUrl = loopback, voiceId = "voice1")
        val netReq = TtsCache.synthesizeRequest("Hello world.", netConfig)

        val ok = TtsCache.fetchSynthesize("$loopback/v1/synthesize", "test-token", netReq, 2000, 5000)
        check(ok.statusCode == 200) { "expected 200, got ${ok.statusCode}" }
        check(ok.audio.contentEquals(okAudio)) { "audio bytes mismatch on 200 path" }
        check(ok.sentenceStatuses == okStatuses) { "X-Sentence-Statuses mismatch: ${ok.sentenceStatuses}" }
        check(ok.region == "eu-west") { "X-Region mismatch: ${ok.region}" }
        check(lastAuth == "Bearer test-token") { "app token not forwarded, got \"$lastAuth\"" }
        println("fetchSynthesize: 200 path with headers and Bearer token ok")

        val partial = TtsCache.fetchSynthesize("$loopback/v1/partial", "test-token", netReq, 2000, 5000)
        check(partial.statusCode == 206) { "expected 206, got ${partial.statusCode}" }
        check(partial.audio.contentEquals(partialAudio)) { "audio bytes mismatch on 206 path" }
        check(partial.sentenceStatuses == partialStatuses) {
            "206 statuses mismatch: ${partial.sentenceStatuses}"
        }
        check(partial.sentenceStatuses.contains("\"status\":\"hit\"")) { "206 statuses missing hit entry" }
        check(partial.sentenceStatuses.contains("\"status\":\"synthesized\"")) {
            "206 statuses missing synthesized entry"
        }
        println("fetchSynthesize: 206 partial path with statuses ok")
    } finally {
        server.stop(0)
    }

    val closedPort = ServerSocket(0).use { it.localPort }
    var failed = false
    try {
        val doomed = TtsCache.synthesizeRequest("Hello.", TtsCacheConfig(baseUrl = "http://127.0.0.1:$closedPort", voiceId = "v"))
        TtsCache.fetchSynthesize("http://127.0.0.1:$closedPort/v1/synthesize", "t", doomed, 500, 500)
    } catch (e: IOException) {
        failed = true
    }
    check(failed) { "expected IOException on connection failure" }
    println("fetchSynthesize: connection-failure path raises IOException ok")
}
