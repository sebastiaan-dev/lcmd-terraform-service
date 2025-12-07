package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	lzcsdk "gitee.com/linakesi/lzc-sdk/lang/go"
	"gitee.com/linakesi/lzc-sdk/lang/go/common"
	"gitee.com/linakesi/lzc-sdk/lang/go/sys"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	bolt "go.etcd.io/bbolt"
)

type Server struct {
	gw    *lzcsdk.APIGateway
	store *LPKStore
}

func main() {
	ctx := context.Background()
	gw, err := lzcsdk.NewAPIGateway(ctx)
	if err != nil {
		log.Fatalf("init gateway: %v", err)
	}
	store, err := NewLPKStore("./lpks")
	if err != nil {
		log.Fatalf("init lpk store: %v", err)
	}
	srv := &Server{gw: gw, store: store}
	r := chi.NewRouter()
	r.Use(middleware.Logger, middleware.Recoverer)

	r.Get("/healthz", srv.health)
	r.Get("/v1/users", srv.listUsers)
	r.Post("/v1/apps", srv.installApp)
	r.Get("/v1/apps", srv.listApps)
	r.Get("/v1/apps/{appid}", srv.getApp)
	r.Delete("/v1/apps/{appid}", srv.deleteApp)
	r.Post("/v1/lpks", srv.uploadLPK)
	r.Get("/v1/lpks", srv.listLPKs)
	r.Get("/v1/lpks/{id}", srv.getLPK)
	r.Get("/v1/lpks/{id}/download", srv.downloadLPK)

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

// LPK registry implementation

type LPKStore struct {
	root  string
	db    *bolt.DB
	mu    sync.RWMutex
	items map[string]*LPKMetadata
}

type LPKMetadata struct {
	ID         string    `json:"id"`
	UID        string    `json:"uid"`
	Name       string    `json:"name"`
	Version    string    `json:"version"`
	SHA256     string    `json:"sha256"`
	Size       int64     `json:"size"`
	Path       string    `json:"-"`
	UploadedAt time.Time `json:"uploaded_at"`
}

var bucketLPK = []byte("lpks")

func NewLPKStore(root string) (*LPKStore, error) {
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, err
	}
	dbPath := filepath.Join(root, "registry.db")
	db, err := bolt.Open(dbPath, 0o600, &bolt.Options{Timeout: time.Second})
	if err != nil {
		return nil, err
	}
	store := &LPKStore{
		root:  root,
		db:    db,
		items: make(map[string]*LPKMetadata),
	}
	if err := db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists(bucketLPK)
		return err
	}); err != nil {
		db.Close()
		return nil, err
	}
	if err := store.loadFromDB(); err != nil {
		db.Close()
		return nil, err
	}
	return store, nil
}

func (s *LPKStore) loadFromDB() error {
	return s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketLPK)
		return b.ForEach(func(k, v []byte) error {
			var meta LPKMetadata
			if err := json.Unmarshal(v, &meta); err != nil {
				return err
			}
			if meta.Path == "" {
				meta.Path = filepath.Join(s.root, meta.ID+".lpk")
			}
			s.items[string(k)] = &meta
			return nil
		})
	})
}

func (s *LPKStore) Close() error {
	return s.db.Close()
}

func (s *LPKStore) Save(uid, name, version string, file multipart.File, filename string) (*LPKMetadata, error) {
	if uid == "" {
		return nil, errors.New("uid is required")
	}
	id, err := randomID()
	if err != nil {
		return nil, err
	}
	if name == "" {
		name = filename
	}
	if version == "" {
		version = time.Now().Format("20060102-150405")
	}
	path := filepath.Join(s.root, id+".lpk")
	out, err := os.Create(path)
	if err != nil {
		return nil, err
	}
	defer out.Close()
	hash := sha256.New()
	size, err := io.Copy(io.MultiWriter(out, hash), file)
	if err != nil {
		return nil, err
	}
	meta := &LPKMetadata{
		ID:         id,
		UID:        uid,
		Name:       name,
		Version:    version,
		SHA256:     hex.EncodeToString(hash.Sum(nil)),
		Size:       size,
		Path:       path,
		UploadedAt: time.Now(),
	}
	if err := s.saveMeta(meta); err != nil {
		return nil, err
	}
	return meta, nil
}

func (s *LPKStore) saveMeta(meta *LPKMetadata) error {
	s.mu.Lock()
	s.items[meta.ID] = meta
	s.mu.Unlock()
	data, err := json.Marshal(meta)
	if err != nil {
		return err
	}
	return s.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bucketLPK).Put([]byte(meta.ID), data)
	})
}

func (s *LPKStore) List(uid string) []*LPKMetadata {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []*LPKMetadata
	for _, meta := range s.items {
		if uid != "" && meta.UID != uid {
			continue
		}
		out = append(out, meta)
	}
	return out
}

func (s *LPKStore) Get(id string) (*LPKMetadata, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	meta, ok := s.items[id]
	return meta, ok
}

func randomID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}

func (s *Server) uploadLPK(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(128 << 20); err != nil {
		http.Error(w, "invalid multipart payload", http.StatusBadRequest)
		return
	}
	uid := r.FormValue("uid")
	name := r.FormValue("name")
	version := r.FormValue("version")
	file, header, err := r.FormFile("package")
	if err != nil {
		http.Error(w, "package file is required", http.StatusBadRequest)
		return
	}
	defer file.Close()
	meta, err := s.store.Save(uid, name, version, file, header.Filename)
	if err != nil {
		httpError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"id":           meta.ID,
		"uid":          meta.UID,
		"name":         meta.Name,
		"version":      meta.Version,
		"sha256":       meta.SHA256,
		"size":         meta.Size,
		"download_url": buildDownloadURL(r, meta.ID),
		"uploaded_at":  meta.UploadedAt,
	})
}

func (s *Server) listLPKs(w http.ResponseWriter, r *http.Request) {
	uid := r.URL.Query().Get("uid")
	items := s.store.List(uid)
	resp := make([]map[string]any, 0, len(items))
	for _, meta := range items {
		resp = append(resp, map[string]any{
			"id":           meta.ID,
			"uid":          meta.UID,
			"name":         meta.Name,
			"version":      meta.Version,
			"sha256":       meta.SHA256,
			"size":         meta.Size,
			"download_url": buildDownloadURL(r, meta.ID),
			"uploaded_at":  meta.UploadedAt,
		})
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) getLPK(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	meta, ok := s.store.Get(id)
	if !ok {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id":           meta.ID,
		"uid":          meta.UID,
		"name":         meta.Name,
		"version":      meta.Version,
		"sha256":       meta.SHA256,
		"size":         meta.Size,
		"download_url": buildDownloadURL(r, meta.ID),
		"uploaded_at":  meta.UploadedAt,
	})
}

func (s *Server) downloadLPK(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	meta, ok := s.store.Get(id)
	if !ok {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	f, err := os.Open(meta.Path)
	if err != nil {
		httpError(w, err)
		return
	}
	defer f.Close()
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s.lpk\"", strings.ReplaceAll(meta.Name, "\"", "")))
	_, _ = io.Copy(w, f)
}

func buildDownloadURL(r *http.Request, id string) string {
	scheme := "http"
	if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
		scheme = proto
	} else if r.TLS != nil {
		scheme = "https"
	}
	host := r.Host
	if host == "" {
		host = "localhost"
	}
	return fmt.Sprintf("%s://%s/v1/lpks/%s/download", scheme, host, id)
}
