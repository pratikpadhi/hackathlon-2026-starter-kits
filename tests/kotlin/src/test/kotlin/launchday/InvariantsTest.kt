package launchday

import java.net.URI
import java.net.http.HttpClient
import java.net.http.HttpRequest
import java.net.http.HttpResponse
import java.time.Duration
import kotlin.test.AfterTest
import kotlin.test.Test
import kotlin.test.fail

class ApiClient {
    val api = System.getenv("API_URL") ?: "http://127.0.0.1:8081"
    val auth = System.getenv("AUTHORITY_URL") ?: "http://127.0.0.1:9000"
    private val http = HttpClient.newBuilder().connectTimeout(Duration.ofSeconds(4)).build()

    fun reset() {
        post("$api/admin/reset", "{}")
        post("$auth/admin/reset", "{}")
    }

    fun setMode(mode: String) {
        post("$auth/admin/mode", "{\"mode\":\"$mode\"}")
    }

    fun reserve(user: String, item: String, key: String, qty: Int): HttpResponse<String> {
        val body = "{\"itemId\":\"$item\",\"userId\":\"$user\",\"qty\":$qty}"
        val req = HttpRequest.newBuilder(URI.create("$api/reservations"))
            .timeout(Duration.ofSeconds(8))
            .header("Content-Type", "application/json")
            .header("Idempotency-Key", key)
            .POST(HttpRequest.BodyPublishers.ofString(body))
            .build()
        return http.send(req, HttpResponse.BodyHandlers.ofString())
    }

    private fun post(url: String, body: String) {
        http.send(
            HttpRequest.newBuilder(URI.create(url))
                .header("Content-Type", "application/json")
                .POST(HttpRequest.BodyPublishers.ofString(body)).build(),
            HttpResponse.BodyHandlers.ofString()
        )
    }
}

class InvariantsTest {
    private val c = ApiClient()

    @AfterTest
    fun restore() {
        c.setMode("healthy")
    }

    @Test fun i1IdempotentReplay() {
        c.reset()
        fail("implement I1: 50 sequential and 50 concurrent replays of the same key")
    }

    @Test fun i2NeverOversell() {
        c.reset()
        fail("implement I2: 200 concurrent reservations against stock 50")
    }

    @Test fun i3ReplayCompleteness() {
        c.reset()
        fail("implement I3: down → pending → healthy → no stragglers")
    }

    @Test fun i4StatusMonotonic() {
        c.reset()
        fail("implement I4")
    }

    @Test fun i5KeyMismatch() {
        c.reset()
        fail("implement I5")
    }

    @Test fun i6HealthHonesty() {
        c.reset()
        fail("implement I6")
    }
}
