package main
import (
    "fmt"
    "net/http/httptest"
    h "github.com/awantoch/beemflow/http"
    "os"
)
func main(){
    os.Setenv("BEEMFLOW_ENDPOINTS", "system")
    req := httptest.NewRequest("GET", "/flows", nil)
    w := httptest.NewRecorder()
    h.ServerlessHandler(w, req)
    fmt.Println("status", w.Code)
    fmt.Println("body", w.Body.String())
}

