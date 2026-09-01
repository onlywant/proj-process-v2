package app

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func call(t *testing.T, s *Server, method, path, bearer string, value any) *httptest.ResponseRecorder {
	t.Helper()
	var body []byte
	if value != nil {
		body, _ = json.Marshal(value)
	}
	r := httptest.NewRequest(method, path, bytes.NewReader(body))
	if bearer != "" {
		r.Header.Set("Authorization", "Bearer "+bearer)
	}
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, r)
	return w
}
func envelope(t *testing.T, w *httptest.ResponseRecorder, out any) {
	t.Helper()
	var e struct {
		Data  json.RawMessage `json:"data"`
		Error string          `json:"error"`
	}
	if json.Unmarshal(w.Body.Bytes(), &e) != nil {
		t.Fatal(w.Body.String())
	}
	if e.Error != "" {
		t.Fatal(e.Error)
	}
	if out != nil && json.Unmarshal(e.Data, out) != nil {
		t.Fatal(string(e.Data))
	}
}
func loginToken(t *testing.T, s *Server, username, password string) string {
	var v struct {
		Token string `json:"token"`
	}
	w := call(t, s, http.MethodPost, "/api/login", "", map[string]string{"username": username, "password": password})
	envelope(t, w, &v)
	return v.Token
}

func TestCoreProjectWorkflow(t *testing.T) {
	s, err := New(t.TempDir()+"/app.db", "")
	if err != nil {
		t.Fatal(err)
	}
	admin := loginToken(t, s, "admin", "admin123")
	var p Project
	w := call(t, s, http.MethodPost, "/api/projects", admin, Project{Province: "江苏", Name: "新工程测试", Source: "测试来源", ProjectYear: "2026", ProjectNo: "A-01", Stage: "申报中", Health: "正常", Progress: "首次记录", UpdateCycle: "每周"})
	if w.Code != 201 {
		t.Fatalf("create=%d %s", w.Code, w.Body)
	}
	envelope(t, w, &p)
	if p.ID == "" || p.CreatedAt == "" {
		t.Fatalf("missing project identity: %+v", p)
	}
	if p.Source != "测试来源" {
		t.Fatalf("source=%q", p.Source)
	}
	second := Project{Province: "江苏", Name: "新工程测试", ProjectYear: "2026", ProjectNo: "A-02", Stage: "申报中", Health: "正常", Progress: "第二个项目", UpdateCycle: "每周"}
	w = call(t, s, http.MethodPost, "/api/projects", admin, second)
	if w.Code != 201 {
		t.Fatalf("number-disambiguated create=%d", w.Code)
	}
	p.Progress = "第二次记录"
	p.Version = 1
	w = call(t, s, http.MethodPut, "/api/projects/"+p.ID+"/progress", admin, p)
	if w.Code != 200 {
		t.Fatalf("progress=%d %s", w.Code, w.Body)
	}
	var d struct {
		Project Project
		History []History
	}
	w = call(t, s, http.MethodGet, "/api/projects/"+p.ID, admin, nil)
	envelope(t, w, &d)
	issue := map[string]any{"version": p.Version, "health": "有问题", "problemDescription": "需要协调", "problemTags": []string{"进度滞后"}, "progress": "问题进展", "stage": p.Stage, "updateCycle": p.UpdateCycle}
	w = call(t, s, http.MethodPost, "/api/projects/"+p.ID+"/progress", admin, issue)
	if w.Code != 200 {
		t.Fatalf("issue progress=%d %s", w.Code, w.Body)
	}
	if len(d.History) != 2 {
		t.Fatalf("history=%d", len(d.History))
	}
	w = call(t, s, http.MethodGet, "/api/projects/"+p.ID, admin, nil)
	envelope(t, w, &d)
	if d.Project.Progress != "问题进展" || d.Project.Name != p.Name || d.Project.Source != p.Source {
		t.Fatalf("project after progress=%+v", d.Project)
	}
	importBody := map[string]any{"projects": []Project{{Province: "浙江", Name: "导入测试", ProjectYear: "2026", Health: "正常", Progress: "导入内容", UpdateCycle: "每周"}}}
	if w := call(t, s, http.MethodPost, "/api/imports/projects", admin, importBody); w.Code != 200 {
		t.Fatalf("import preview=%d %s", w.Code, w.Body)
	}
	importBody["confirm"] = true
	if w := call(t, s, http.MethodPost, "/api/imports/projects", admin, importBody); w.Code != 200 {
		t.Fatalf("import confirm=%d %s", w.Code, w.Body)
	}
	if w := call(t, s, http.MethodGet, "/api/exports/projects", admin, nil); w.Code != 200 {
		t.Fatalf("export=%d", w.Code)
	}
}

func TestPermissionsAndPassword(t *testing.T) {
	s, err := New(t.TempDir()+"/app.db", "")
	if err != nil {
		t.Fatal(err)
	}
	admin := loginToken(t, s, "admin", "admin123")
	ordinary := loginToken(t, s, "yuquanqing", "yuquanqing")
	if w := call(t, s, http.MethodGet, "/api/config", ordinary, nil); w.Code != 403 {
		t.Fatalf("ordinary config=%d", w.Code)
	}
	if w := call(t, s, http.MethodPut, "/api/me/password", admin, map[string]string{"currentPassword": "admin123", "newPassword": "newadmin123"}); w.Code != 200 {
		t.Fatalf("password=%d %s", w.Code, w.Body)
	}
	if w := call(t, s, http.MethodPost, "/api/login", "", map[string]string{"username": "admin", "password": "newadmin123"}); w.Code != 200 {
		t.Fatalf("new password login=%d", w.Code)
	}
	if w := call(t, s, http.MethodPut, "/api/users/u-yuquanqing", admin, map[string]string{"status": "停用"}); w.Code != 200 {
		t.Fatalf("disable=%d", w.Code)
	}
	if w := call(t, s, http.MethodGet, "/api/dictionaries", ordinary, nil); w.Code != 401 {
		t.Fatalf("disabled user status=%d", w.Code)
	}
}

func TestAdminCanConfigureProjectTypes(t *testing.T) {
	s, err := New(t.TempDir()+"/app.db", "")
	if err != nil {
		t.Fatal(err)
	}
	admin := loginToken(t, s, "admin", "admin123")
	var config Dictionary
	w := call(t, s, http.MethodPut, "/api/config", admin, Dictionary{
		Provinces: []string{"江苏"}, ProjectTypes: []string{"专项项目", "常规项目"},
		Stages: []string{"策划中"}, HealthStatuses: []string{"正常"},
		UpdateCycles: []string{"每周"}, NearDays: 3,
	})
	if w.Code != 200 {
		t.Fatalf("update config=%d %s", w.Code, w.Body)
	}
	w = call(t, s, http.MethodGet, "/api/dictionaries", admin, nil)
	envelope(t, w, &config)
	if len(config.ProjectTypes) != 2 || config.ProjectTypes[0] != "专项项目" {
		t.Fatalf("project types not persisted: %#v", config.ProjectTypes)
	}
	invalid := Dictionary{Provinces: []string{"江苏"}, Stages: []string{"策划中"}, HealthStatuses: []string{"正常"}, UpdateCycles: []string{"每周"}}
	if w := call(t, s, http.MethodPut, "/api/config", admin, invalid); w.Code != 400 {
		t.Fatalf("empty project types accepted: %d", w.Code)
	}
}

func TestDefaultEditorUsersAndAdminReset(t *testing.T) {
	s, err := New(t.TempDir()+"/app.db", "")
	if err != nil {
		t.Fatal(err)
	}
	admin := loginToken(t, s, "admin", "admin123")
	var users []User
	w := call(t, s, http.MethodGet, "/api/users", admin, nil)
	envelope(t, w, &users)
	wanted := []string{"yuquanqing", "tianyu", "qinxiaomin", "luyuhua", "wangchen"}
	for _, username := range wanted {
		found := false
		for _, u := range users {
			if u.Username == username {
				found = true
				if u.Role != "user" || u.Status != "启用" {
					t.Fatalf("default user %s has role=%s status=%s", username, u.Role, u.Status)
				}
			}
		}
		if !found {
			t.Fatalf("default user %s is missing", username)
		}
		loginToken(t, s, username, username)
	}
	if w := call(t, s, http.MethodPut, "/api/users/u-tianyu", admin, map[string]string{"password": "newtianyu123"}); w.Code != 200 {
		t.Fatalf("admin reset=%d %s", w.Code, w.Body)
	}
	loginToken(t, s, "tianyu", "newtianyu123")
}

func TestAdminCanClearProjectsOnly(t *testing.T) {
	s, err := New(t.TempDir()+"/app.db", "")
	if err != nil {
		t.Fatal(err)
	}
	admin := loginToken(t, s, "admin", "admin123")
	ordinary := loginToken(t, s, "yuquanqing", "yuquanqing")
	w := call(t, s, http.MethodPost, "/api/projects", admin, Project{Province: "江苏", Name: "清空测试", ProjectYear: "2026", Health: "正常", Progress: "测试", UpdateCycle: "每周"})
	if w.Code != 201 {
		t.Fatalf("create=%d %s", w.Code, w.Body)
	}
	if w := call(t, s, http.MethodDelete, "/api/projects", ordinary, nil); w.Code != 403 {
		t.Fatalf("ordinary clear=%d", w.Code)
	}
	if w := call(t, s, http.MethodDelete, "/api/projects", admin, nil); w.Code != 200 {
		t.Fatalf("admin clear=%d %s", w.Code, w.Body)
	}
	var listed struct {
		Items []Project `json:"items"`
	}
	w = call(t, s, http.MethodGet, "/api/projects?page=1&pageSize=20", admin, nil)
	envelope(t, w, &listed)
	if len(listed.Items) != 0 {
		t.Fatalf("projects remain after clear: %d", len(listed.Items))
	}
	if w := call(t, s, http.MethodGet, "/api/users", admin, nil); w.Code != 200 {
		t.Fatalf("users removed with projects=%d", w.Code)
	}
}

func TestAdminCanDeleteSingleProjectOnly(t *testing.T) {
	s, err := New(t.TempDir()+"/app.db", "")
	if err != nil {
		t.Fatal(err)
	}
	admin := loginToken(t, s, "admin", "admin123")
	ordinary := loginToken(t, s, "yuquanqing", "yuquanqing")
	w := call(t, s, http.MethodPost, "/api/projects", admin, Project{Province: "江苏", Name: "单项删除测试", ProjectYear: "2026", Health: "正常", Progress: "测试", UpdateCycle: "每周"})
	if w.Code != 201 {
		t.Fatalf("create=%d %s", w.Code, w.Body)
	}
	var p Project
	envelope(t, w, &p)
	if w := call(t, s, http.MethodDelete, "/api/projects/"+p.ID, ordinary, nil); w.Code != 403 {
		t.Fatalf("ordinary delete=%d", w.Code)
	}
	if w := call(t, s, http.MethodDelete, "/api/projects/"+p.ID, admin, nil); w.Code != 200 {
		t.Fatalf("admin delete=%d %s", w.Code, w.Body)
	}
	if w := call(t, s, http.MethodGet, "/api/projects/"+p.ID, admin, nil); w.Code != 404 {
		t.Fatalf("deleted detail=%d", w.Code)
	}
	if w := call(t, s, http.MethodDelete, "/api/projects/"+p.ID, admin, nil); w.Code != 404 {
		t.Fatalf("repeat delete=%d", w.Code)
	}
}
