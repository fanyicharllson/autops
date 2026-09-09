import http from "k6/http";
import { check, sleep } from "k6";

// Basic spike simulation against the AutOps proxy MVP.
export const options = {
  stages: [
    { duration: "10s", target: 10 }, // warm up
    { duration: "30s", target: 200 }, // traffic spike
    { duration: "20s", target: 200 }, // sustain
    { duration: "10s", target: 0 }, // ramp down
  ],
};

export default function () {
  const res = http.get("http://localhost:8080/");
  check(res, {
    "status is 2xx or upstream error (proxy up)": (r) => r.status > 0,
  });
  sleep(0.1);
}
