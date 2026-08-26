package signing

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

// goldenPrivateKey is intentionally public and exists only to keep the
// signature compatibility vector deterministic. It must never be trusted.
const goldenPrivateKey = `-----BEGIN PRIVATE KEY-----
MIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQDv3D+Q/hZDLTZ1
gUqKa0TNEbQLZhiDyyfmozZt8S+1r6RE4Q6K4aNAnSemS5W26yR6x7lAiTg80s6w
LwcXzJE4rbPzdvokCZI+rIHI1pRwXk4Te+KmCHnrQm1roaM7wiv3B3Nuo5sn+4zk
X1raCydpvwcFQL+WCcqeinZV8N2uv9aaYdL8W4Dw0I6QQMDk9ZEtmipAQCZipcHO
QmK/5XoTvbPqXH5+IwtieB7ecofaDa6v73l3vy4lDR7I4auoJzgGpxh3tQYPn3oj
IVh41+Etbz5eukeJs4ucb1lFPwVudJ2ESC+0BYaR/B/c/u1AY5ImKUtGduy6D9/a
khwLi4PnAgMBAAECggEAYgLKGLmyqB2D+MzthBUkBnW678N2tffgJI7BsXgR6jhM
s/aQpjBgMIlnB70v/pAkFmWhLZ1ElRoA6h41VES7fIECHLSbrvkYQLLlz4bbHfVz
CuQe74BcLUU2B/qDBGqp42Wwhd+liVdxMWpDagCPyNaNjFhyUOFMYa8rfp+PPhWM
FdiWsq9H3DxM7HJDzhEeqnN6fJHN+OgMKQP1rilg8mHYlBBGh52NGfG8/WRh38VL
6NHpUvmLUmW4d5CLzMbdMX/M49sy2/MfalRs22t5G19qs82EMiqPJ1Fp3d06cb6z
VzDUGgXJn0hkPC0AgyeI8fQ3X12BTQiPQPbeFH+jgQKBgQD75tYDw9TrxESeTmfc
cqiDw4vLP1ut4X9JYIqo8XJtARyE2dNH4xATEWCfpfdWTraDUJRc3RoFqv4Y2Y0H
4Udk80iNu25QMhczbFd+7q9+dKMWyjNNKvahUlRN9IYzzds7PiKMkKFzwIEyObS+
q91d2bJqB0MlacrF97+jYj/tZwKBgQDzw0KnkehgYZiIHPSDJW23FM/u+YJ8I8+F
WQPsUWVFGoSrB2hzqx55C7SPnmlB3GnWZruZhRNFkSsiZBTP2lTKan/5BcJ3X4cu
1a+8bqz4uOXK6GVY9wE796sP1ZbYlkJVc28MCplqF26cUaJ1NGNeSz08yJkbtHSS
M8pXK7MlgQKBgEcfff3CJTQMTnOEH78WgI3zgFz6nqARsya8o2ngAjbUwYChIA6E
Bd3cYjOxYIx13QJmlf1CUD6ZcPUDN9apvMa8Chg6e4MZIYPbazjjm5lQxVuB68o1
/zy9SiBYkiu+S9AGUyrtCyjriY7szCOp0u7UsJNPSZ4eqKoMsYcoDnSxAoGBAKjE
YZue22IPcAmc8nTyLvn4wNXVjc/hA0ZxkNPGrvSfHYdEA79BlEF+Jy7gIEPnFKfK
TMxHZEZf8ZtH61jU8quJ/LwujqsTSobUj21Iux0g9of7Sl/D8+jO2nKGEIA32AlN
eG6/z+OyAXTc2DuJX9rjAKzavZZ+485taQPdT5UBAoGAHF5doHoD2CCDA36d+xXS
6+p7eoww14DpEMfjRSGdZ8gayKevVoW4JZ8lDIynTa0EUqtpJ1nNYZpu8jcZsPta
iA2HUEeEe6+IOGaEZaGSdw7b3hrmLb/Dimz7GDTA0ybNOVo7miRccQuWD2gmUsCO
BVoqay12XRMKK9ozSdrVS2E=
-----END PRIVATE KEY-----`

func TestGoldenSignatureVector(t *testing.T) {
	t.Parallel()

	const (
		messageID         = "01ARZ3NDEKTSV4RRFFQ69G5FAV"
		timestamp         = "2026-08-24T10:15:30.123Z"
		rawBody           = `{"content":"golden"}`
		expectedInput     = `01ARZ3NDEKTSV4RRFFQ69G5FAV.2026-08-24T10:15:30.123Z.{"content":"golden"}`
		expectedDigest    = "eb8eb00834e3eb190dd47591d91f702d894d0b51de8418a26e33411adb3f7543"
		expectedSignature = "AZmEfE/Uy+Lvsn9TmRYpGPHiXywMj/AHkvuyfcz917cN3MXp19pZfEyx9ahXLRBTQx9yHMky7LM34/fJ8oFYIBuYpiPjA1POIeMvwUerNvNmhEpULEpdp5p/p4BAn6+IaUS+N9Ou1nAE3DOR3ZEtzJPU52A2MJFdrniT6HKGziESnrJd1HbWkx4pol6rC/kLI7NNakFX8GOXTAFmdzpTO6xSP8cV1N58QyavDGuKv/IPHQCfogU1dP7C0HlPcjmfL42CVPFbBrUs3E11jlq2lsej7Uk6tkJ2RmNr591/xzqUoK+/cset5h/Juypd3DaTVb2cvzKUhDg9n30WAxrwaw=="
	)

	privateKey, err := ParsePrivateKey([]byte(goldenPrivateKey))
	if err != nil {
		t.Fatal(err)
	}
	input := SignatureInput(messageID, timestamp, []byte(rawBody))
	if string(input) != expectedInput {
		t.Fatalf("signature input = %q", input)
	}
	digest := sha256.Sum256(input)
	signature, err := Sign(privateKey, messageID, timestamp, []byte(rawBody))
	if err != nil {
		t.Fatal(err)
	}
	if got := hex.EncodeToString(digest[:]); got != expectedDigest {
		t.Fatalf("digest = %s, want %s", got, expectedDigest)
	}
	if signature != expectedSignature {
		t.Fatalf("signature = %s, want %s", signature, expectedSignature)
	}
}
