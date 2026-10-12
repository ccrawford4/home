package gpupool

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGCEMIGGetAndResize(t *testing.T) {
	const path = "/compute/v1/projects/home-473419/zones/us-central1-a/instanceGroupManagers/gpu-pool"
	var resized string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == path:
			w.Write([]byte(`{"name":"gpu-pool","targetSize":1,"status":{"isStable":false}}`))
		case r.Method == http.MethodPost && r.URL.Path == path+"/resize":
			resized = r.URL.Query().Get("size")
			w.Write([]byte(`{"kind":"compute#operation","status":"RUNNING"}`))
		default:
			http.Error(w, "unexpected "+r.Method+" "+r.URL.Path, http.StatusNotFound)
		}
	}))
	defer srv.Close()

	m := newGCEMIG(srv.Client(), srv.URL, Config{Project: "home-473419", Zone: "us-central1-a", MIG: "gpu-pool"})
	ctx := context.Background()

	st, err := m.Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if st.TargetSize != 1 || st.Stable {
		t.Fatalf("Get = %+v, want target 1, not stable", st)
	}
	if err := m.Resize(ctx, 0); err != nil {
		t.Fatal(err)
	}
	if resized != "0" {
		t.Fatalf("resize size = %q, want 0", resized)
	}
}

func TestGCEMIGReportsAPIErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":{"message":"Required 'compute.instanceGroupManagers.update' permission"}}`, http.StatusForbidden)
	}))
	defer srv.Close()

	m := newGCEMIG(srv.Client(), srv.URL, Config{Project: "p", Zone: "z", MIG: "m"})
	err := m.Resize(context.Background(), 1)
	if err == nil || !strings.Contains(err.Error(), "403") || !strings.Contains(err.Error(), "permission") {
		t.Fatalf("Resize error = %v, want the 403 and its message", err)
	}
}
