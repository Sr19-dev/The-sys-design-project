package main
import "log/slog"
import "os"
import "net/http"
func main(){
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func (w http.ResponseWriter, r *http.Request ) {w.Write([]byte("ok"))} )
	http.ListenAndServe(":8080", mux)
}