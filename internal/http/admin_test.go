package http

import "testing"

func TestAllowedDocumentUpload(t *testing.T) {
	t.Parallel()

	tests := []struct {
		filename    string
		contentType string
		want        bool
	}{
		{filename: "scan.pdf", contentType: "application/pdf", want: true},
		{filename: "scan.jpg", contentType: "image/jpeg", want: true},
		{filename: "scan.png", contentType: "image/png", want: true},
		{filename: "scan.exe", contentType: "image/jpeg", want: false},
		{filename: "scan.jpg", contentType: "application/octet-stream", want: false},
	}
	for _, test := range tests {
		if got := allowedDocumentUpload(test.filename, test.contentType); got != test.want {
			t.Fatalf("allowed upload for %s %s = %t, want %t", test.filename, test.contentType, got, test.want)
		}
	}
}
