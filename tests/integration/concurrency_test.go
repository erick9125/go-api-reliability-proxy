package integration_test

import (
	"net/http"
	"sync"
	"testing"

	"github.com/erick9125/go-api-reliability-proxy/internal/proxy"
)

func TestConcurrentPassthrough(t *testing.T) {
	upstream := newUpstream(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	p := newProxyServer(t, upstream.URL, nil, proxy.Options{})

	const n = 100
	var wg sync.WaitGroup
	errCh := make(chan error, n)
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			resp, err := http.Get(p.URL + "/hello")
			if err != nil {
				errCh <- err
				return
			}
			resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				errCh <- errStatus(resp.StatusCode)
			}
		}()
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Fatal(err)
	}
}

type statusError int

func errStatus(code int) error {
	return statusError(code)
}

func (e statusError) Error() string {
	return http.StatusText(int(e))
}
