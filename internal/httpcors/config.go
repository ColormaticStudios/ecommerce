package httpcors

var allowMethods = []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"}

var allowHeaders = []string{
	"Origin",
	"Content-Type",
	"Content-Length",
	"Accept-Encoding",
	"Authorization",
	"X-CSRF-Token",
	"Idempotency-Key",
	"Tus-Resumable",
	"Upload-Length",
	"Upload-Metadata",
	"Upload-Offset",
}

var exposeHeaders = []string{
	"Content-Length",
	"Location",
}

func AllowMethods() []string {
	return append([]string(nil), allowMethods...)
}

func AllowHeaders() []string {
	return append([]string(nil), allowHeaders...)
}

func ExposeHeaders() []string {
	return append([]string(nil), exposeHeaders...)
}
