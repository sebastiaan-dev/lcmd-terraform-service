package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"

	lzcsdk "gitee.com/linakesi/lzc-sdk/lang/go"
	"gitee.com/linakesi/lzc-sdk/lang/go/common"
	"gitee.com/linakesi/lzc-sdk/lang/go/sys"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type Server struct {
	gw *lzcsdk.APIGateway
}

func main() {
	ctx := context.Background()
	gw, err := lzcsdk.NewAPIGateway(ctx)
	if err != nil {
		log.Fatalf("init gateway: %v", err)
	}
	srv := &Server{gw: gw}
	r := chi.NewRouter()
	r.Use(middleware.Logger, middleware.Recoverer)

	r.Get("/healthz", srv.health)
	r.Get("/v1/users", srv.listUsers)
	r.Post("/v1/apps", srv.installApp)
	r.Get("/v1/apps", srv.listApps)
	r.Get("/v1/apps/{appid}", srv.getApp)
	r.Delete("/v1/apps/{appid}", srv.deleteApp)

	httpSrv := &http.Server{
		Addr:    ":9443",
		Handler: r,
	}
	log.Println("API listening on :9443 with mTLS")
	log.Fatal(httpSrv.ListenAndServe())
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) listUsers(w http.ResponseWriter, r *http.Request) {
	resp, err := s.gw.Users.ListUIDs(r.Context(), &common.ListUIDsRequest{})
	if err != nil {
		httpError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp.GetUids())
}

type installReq struct {
	UID       string `json:"uid"`
	LPKURL    string `json:"lpk_url"`
	Wait      bool   `json:"wait"`
	Ephemeral bool   `json:"ephemeral"`
}

func (s *Server) installApp(w http.ResponseWriter, r *http.Request) {
	var reqBody installReq
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}
	ctx := lzcsdk.WithRealUID(r.Context(), reqBody.UID)
	waitFlag := reqBody.Wait
	resp, err := s.gw.PkgManager.InstallLPK(ctx, &sys.InstallLPKRequest{
		LpkUrl:       reqBody.LPKURL,
		WaitUnitDone: &waitFlag,
	})
	if err != nil {
		httpError(w, err)
		return
	}
	task := resp.GetTaskInfo()
	apps, err := s.gw.PkgManager.QueryApplication(ctx, &sys.QueryApplicationRequest{
		DeployIds: []string{task.GetRealPkgId()},
	})
	if err != nil || len(apps.GetInfoList()) == 0 {
		http.Error(w, "app info not found", http.StatusBadGateway)
		return
	}
	app := apps.GetInfoList()[0]
	writeJSON(w, http.StatusCreated, map[string]any{
		"appid":     app.GetAppid(),
		"deploy_id": app.GetDeployId(),
		"lpk_id":    task.GetRealPkgId(),
		"title":     app.GetTitle(),
		"version":   app.GetVersion(),
		"domain":    app.GetDomain(),
		"owner":     app.GetOwner(),
	})
}

func (s *Server) listApps(w http.ResponseWriter, r *http.Request) {
	uid := r.URL.Query().Get("uid")
	ctx := lzcsdk.WithRealUID(r.Context(), uid)
	resp, err := s.gw.PkgManager.QueryApplication(ctx, &sys.QueryApplicationRequest{})
	if err != nil {
		httpError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp.GetInfoList())
}

func (s *Server) getApp(w http.ResponseWriter, r *http.Request) {
	uid := r.URL.Query().Get("uid")
	appid := chi.URLParam(r, "appid")
	ctx := lzcsdk.WithRealUID(r.Context(), uid)
	resp, err := s.gw.PkgManager.QueryApplication(ctx, &sys.QueryApplicationRequest{
		DeployIds: []string{appid},
	})
	if err != nil || len(resp.GetInfoList()) == 0 {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, resp.GetInfoList()[0])
}

func (s *Server) deleteApp(w http.ResponseWriter, r *http.Request) {
	uid := r.URL.Query().Get("uid")
	clear := r.URL.Query().Get("clear_data") == "true"
	appid := chi.URLParam(r, "appid")
	ctx := lzcsdk.WithRealUID(r.Context(), uid)
	_, err := s.gw.PkgManager.Uninstall(ctx, &sys.UninstallRequest{
		Appid:     appid,
		ClearData: clear,
	})
	if err != nil {
		httpError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func httpError(w http.ResponseWriter, err error) {
	log.Println("API error:", err)
	http.Error(w, err.Error(), http.StatusBadGateway)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
