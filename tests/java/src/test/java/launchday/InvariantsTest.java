package launchday;

import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.Test;

import static org.junit.jupiter.api.Assertions.fail;

class InvariantsTest {
    private final ApiClient c = new ApiClient();

    @AfterEach
    void restore() throws Exception {
        c.setMode("healthy");
    }

    @Test
    void i1IdempotentReplay() throws Exception {
        c.reset();
        fail("implement I1: 50 sequential and 50 concurrent replays of the same key");
    }

    @Test
    void i2NeverOversell() throws Exception {
        c.reset();
        fail("implement I2: 200 concurrent reservations against stock 50");
    }

    @Test
    void i3ReplayCompleteness() throws Exception {
        c.reset();
        fail("implement I3: down → pending → healthy → no stragglers");
    }

    @Test
    void i4StatusMonotonic() throws Exception {
        c.reset();
        fail("implement I4: pending only moves to confirmed or reversed");
    }

    @Test
    void i5KeyMismatch() throws Exception {
        c.reset();
        fail("implement I5: same key, different body → 422");
    }

    @Test
    void i6HealthHonesty() throws Exception {
        c.reset();
        fail("implement I6: /health.mode follows authority toggles within 5s");
    }
}
