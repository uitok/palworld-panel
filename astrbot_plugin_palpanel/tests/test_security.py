import time
import unittest

from astrbot_plugin_palpanel.security import body_bytes, signature, verify_headers


class SecurityTests(unittest.TestCase):
    def test_signature_accepts_valid_request_and_rejects_tampering(self):
        body = body_bytes({"player_uid": "uid-1", "nickname": "测试"})
        timestamp = str(int(time.time()))
        nonce = "nonce-1"
        headers = {
            "X-PalPanel-Timestamp": timestamp,
            "X-PalPanel-Nonce": nonce,
            "X-PalPanel-Signature": signature("secret", "POST", "/v1/catalog/sync", timestamp, nonce, body),
        }
        self.assertEqual(verify_headers("secret", "POST", "/v1/catalog/sync", headers, body), (True, nonce))
        self.assertFalse(verify_headers("secret", "POST", "/v1/catalog/sync", headers, body + b"x")[0])

    def test_signature_rejects_expired_timestamp(self):
        timestamp = str(int(time.time()) - 120)
        headers = {
            "X-PalPanel-Timestamp": timestamp,
            "X-PalPanel-Nonce": "old",
            "X-PalPanel-Signature": signature("secret", "POST", "/v1/test", timestamp, "old", b"{}"),
        }
        self.assertFalse(verify_headers("secret", "POST", "/v1/test", headers, b"{}")[0])


if __name__ == "__main__":
    unittest.main()
