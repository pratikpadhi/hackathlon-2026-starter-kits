package launchday;

import java.net.URI;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.time.Duration;

public class ApiClient {
    public final String api = env("API_URL", "http://127.0.0.1:8081");
    public final String auth = env("AUTHORITY_URL", "http://127.0.0.1:9000");
    private final HttpClient http = HttpClient.newBuilder().connectTimeout(Duration.ofSeconds(4)).build();

    public HttpResponse<String> reserve(String user, String item, String key, int qty) throws Exception {
        String body = "{\"itemId\":\"" + item + "\",\"userId\":\"" + user + "\",\"qty\":" + qty + "}";
        HttpRequest.Builder b = HttpRequest.newBuilder(URI.create(api + "/reservations"))
                .timeout(Duration.ofSeconds(8))
                .header("Content-Type", "application/json")
                .POST(HttpRequest.BodyPublishers.ofString(body));
        if (key != null) b.header("Idempotency-Key", key);
        return http.send(b.build(), HttpResponse.BodyHandlers.ofString());
    }

    public void reset() throws Exception {
        post(api + "/admin/reset", "{}");
        post(auth + "/admin/reset", "{}");
    }

    public void setMode(String mode) throws Exception {
        post(auth + "/admin/mode", "{\"mode\":\"" + mode + "\"}");
    }

    public HttpResponse<String> health() throws Exception {
        return http.send(HttpRequest.newBuilder(URI.create(api + "/health")).GET().build(),
                HttpResponse.BodyHandlers.ofString());
    }

    private void post(String url, String body) throws Exception {
        http.send(HttpRequest.newBuilder(URI.create(url))
                .header("Content-Type", "application/json")
                .POST(HttpRequest.BodyPublishers.ofString(body)).build(), HttpResponse.BodyHandlers.ofString());
    }

    private static String env(String k, String def) {
        String v = System.getenv(k);
        return v == null || v.isEmpty() ? def : v;
    }
}
