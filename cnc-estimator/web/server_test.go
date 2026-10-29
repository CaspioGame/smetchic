package web

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"smetchic/model"
)

const validPart = `[part]
material=aluminum
stock_width=40
stock_length=30
stock_height=20

[feature hole-1]
type=hole
x=10
y=12
diameter=5
depth=8
`

func testHandler(t *testing.T) http.Handler {
	t.Helper()
	handler, err := NewHandler(model.Machine{
		Name:       "Test machine",
		HourlyRate: 1500,
		MaxFeed:    5000,
	})
	if err != nil {
		t.Fatalf("NewHandler() error = %v", err)
	}
	return handler
}

func TestIndexPage(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	response := httptest.NewRecorder()

	testHandler(t).ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("GET / status = %d, want %d", response.Code, http.StatusOK)
	}
	for _, expected := range []string{
		"CNC Estimator",
		`method="POST"`,
		`enctype="multipart/form-data"`,
		`name="part_file"`,
	} {
		if !strings.Contains(response.Body.String(), expected) {
			t.Errorf("GET / body does not contain %q", expected)
		}
	}
}

func TestUploadDisplaysEstimate(t *testing.T) {
	request := uploadRequest(t, "square.part", validPart)
	response := httptest.NewRecorder()

	testHandler(t).ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("POST / status = %d, want %d; body: %s", response.Code, http.StatusOK, response.Body.String())
	}
	for _, expected := range []string{
		"PART", "square.part", "FEATURES", "hole-1", "MACHINE", "Test machine",
		"OPERATIONS", "Drilling", "Drill", "Estimated/test toolpath length",
		"TIME", "COST", "ESTIMATE", "Suggested price (demo markup)",
	} {
		if !strings.Contains(response.Body.String(), expected) {
			t.Errorf("result body does not contain %q", expected)
		}
	}
}

func TestUploadShowsParserError(t *testing.T) {
	invalidPart := strings.Replace(validPart, "stock_width=40", "stock_width=bad", 1)
	request := uploadRequest(t, "invalid.part", invalidPart)
	response := httptest.NewRecorder()

	testHandler(t).ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("POST / invalid part status = %d, want %d", response.Code, http.StatusBadRequest)
	}
	for _, expected := range []string{"Ошибка расчёта", "invalid.part", "stock_width", "положительным числом"} {
		if !strings.Contains(response.Body.String(), expected) {
			t.Errorf("error body does not contain %q", expected)
		}
	}
	if strings.Contains(response.Body.String(), "goroutine") {
		t.Error("error page must not include a stack trace")
	}
}

func TestUploadRequiresFile(t *testing.T) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	response := httptest.NewRecorder()

	testHandler(t).ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("POST / without file status = %d, want %d", response.Code, http.StatusBadRequest)
	}
	if !strings.Contains(response.Body.String(), "Выберите файл .part") {
		t.Errorf("missing file error not shown: %s", response.Body.String())
	}
}

func uploadRequest(t *testing.T, filename, content string) *http.Request {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	file, err := writer.CreateFormFile("part_file", filename)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(file, content); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	return request
}
