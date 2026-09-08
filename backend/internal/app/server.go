package app

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type Server struct {
	db       *sql.DB
	frontend string
	mux      *http.ServeMux
	sessions map[string]User
	mu       sync.RWMutex
}
type User struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Password string `json:"password,omitempty"`
	Name     string `json:"name"`
	Role     string `json:"role"`
	Status   string `json:"status"`
}
type Project struct {
	ID                          string   `json:"id"`
	ProjectNo                   string   `json:"projectNo"`
	Province                    string   `json:"province"`
	Name                        string   `json:"name"`
	Source                      string   `json:"source"`
	Type                        string   `json:"type"`
	SpecificType                string   `json:"specificType"`
	ProjectYear                 string   `json:"projectYear"`
	ResponsibleDepartment       string   `json:"responsibleDepartment"`
	ExpansionOwner              string   `json:"expansionOwner"`
	LeadParticipatingUnits      string   `json:"leadParticipatingUnits"`
	TotalAmount                 float64  `json:"totalAmount"`
	ContractAmount              float64  `json:"contractAmount"`
	SuccessDate                 string   `json:"successDate"`
	ZhixinRole                  string   `json:"zhixinRole"`
	InternalSupportDepartment   string   `json:"internalSupportDepartment"`
	ProvincialSupportDepartment string   `json:"provincialSupportDepartment"`
	GovernmentUnit              string   `json:"governmentUnit"`
	Stage                       string   `json:"stage"`
	Health                      string   `json:"health"`
	Progress                    string   `json:"progress"`
	ProblemTags                 []string `json:"problemTags"`
	ProblemDescription          string   `json:"problemDescription"`
	NextPlan                    string   `json:"nextPlan"`
	UpdateCycle                 string   `json:"updateCycle"`
	CustomCycleDays             int      `json:"customCycleDays"`
	UpdatedAt                   string   `json:"updatedAt"`
	NextUpdateAt                string   `json:"nextUpdateAt"`
	UpdatedBy                   string   `json:"updatedBy"`
	CreatedAt                   string   `json:"createdAt"`
	CreatedBy                   string   `json:"createdBy"`
	Version                     int      `json:"version"`
}
type History struct {
	ID                 int64    `json:"id"`
	ProjectID          string   `json:"projectId"`
	Stage              string   `json:"stage"`
	Health             string   `json:"health"`
	Progress           string   `json:"progress"`
	ProblemTags        []string `json:"problemTags"`
	ProblemDescription string   `json:"problemDescription"`
	NextPlan           string   `json:"nextPlan"`
	Note               string   `json:"note"`
	UpdatedAt          string   `json:"updatedAt"`
	UpdatedBy          string   `json:"updatedBy"`
	CorrectionOf       int64    `json:"correctionOf,omitempty"`
	CorrectionNote     string   `json:"correctionNote,omitempty"`
}
type Dictionary struct {
	Provinces      []string `json:"provinces"`
	ProjectTypes   []string `json:"projectTypes"`
	Stages         []string `json:"stages"`
	HealthStatuses []string `json:"healthStatuses"`
	ProblemTags    []string `json:"problemTags"`
	UpdateCycles   []string `json:"updateCycles"`
	NearDays       int      `json:"nearDays"`
}
type projectInput struct {
	Project
	Note string `json:"note"`
}

var provinces = []string{"北京", "天津", "河北", "冀北", "山西", "蒙东", "辽宁", "吉林", "黑龙江", "上海", "江苏", "浙江", "安徽", "福建", "江西", "山东", "河南", "湖北", "湖南", "广东", "广西", "海南", "重庆", "四川", "贵州", "云南", "西藏", "陕西", "甘肃", "青海", "宁夏", "新疆", "香港", "澳门", "台湾", "中国电科院", "国网经研院", "国网能源院", "国网工研院", "国网信通中心（大数据中心）", "国网特高压公司", "国网直流中心"}
var directUnits = []string{"中国电科院", "国网经研院", "国网能源院", "国网工研院", "国网信通中心（大数据中心）", "国网特高压公司", "国网直流中心"}

func New(path, frontend string) (*Server, error) {
	if absolute, e := filepath.Abs(path); e == nil {
		log.Printf("database: path=%s", absolute)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	s := &Server{db: db, frontend: frontend, mux: http.NewServeMux(), sessions: map[string]User{}}
	if err = s.init(); err != nil {
		return nil, err
	}
	s.routes()
	return s, nil
}
func (s *Server) ListenAndServe(addr string) error { return http.ListenAndServe(addr, cors(s.mux)) }
func (s *Server) init() error {
	_, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS users(id TEXT PRIMARY KEY,username TEXT UNIQUE NOT NULL,password TEXT NOT NULL,name TEXT NOT NULL,role TEXT NOT NULL,status TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS projects(id TEXT PRIMARY KEY,province TEXT NOT NULL,name TEXT NOT NULL,project_year TEXT NOT NULL,project_no TEXT,payload TEXT NOT NULL,updated_at TEXT NOT NULL,next_update_at TEXT NOT NULL,version INTEGER NOT NULL);
CREATE TABLE IF NOT EXISTS histories(id INTEGER PRIMARY KEY AUTOINCREMENT,project_id TEXT NOT NULL,payload TEXT NOT NULL,updated_at TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS config(id INTEGER PRIMARY KEY CHECK(id=1),payload TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS audit_logs(id INTEGER PRIMARY KEY AUTOINCREMENT,project_id TEXT NOT NULL,operator TEXT NOT NULL,changed_at TEXT NOT NULL,payload TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS import_batches(id TEXT PRIMARY KEY,operator TEXT NOT NULL,started_at TEXT NOT NULL,completed_at TEXT,payload TEXT NOT NULL);`)
	if err != nil {
		return err
	}
	// Keep the two-role model: remove obsolete demo accounts and normalize
	// existing editor/viewer accounts to ordinary users.
	if _, err = s.db.Exec("DELETE FROM users WHERE username IN ('editor','viewer'); UPDATE users SET role='user' WHERE role IN ('editor','viewer');"); err != nil {
		return err
	}
	var count int
	s.db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
	// Seed only the administrator and ordinary-user accounts.
	defaults := []struct{ id, n, p, r string }{
		{"admin", "管理员", "admin123", "admin"},
		{"liujingwen", "刘景文", "liujingwen", "user"},
		{"yuquanqing", "蔚泉清", "yuquanqing", "user"},
		{"tianyu", "田羽", "tianyu", "user"},
		{"qinxiaomin", "秦晓敏", "qinxiaomin", "user"},
		{"luyuhua", "卢玉华", "luyuhua", "user"},
		{"wangchen", "王晨", "wangchen", "user"},
	}
	for _, u := range defaults {
		if _, err = s.db.Exec("UPDATE users SET name=?,role=? WHERE username=?", u.n, u.r, u.id); err != nil {
			return err
		}
		var exists int
		s.db.QueryRow("SELECT COUNT(*) FROM users WHERE username=?", u.id).Scan(&exists)
		if exists > 0 {
			// Keep the documented built-in administrator credentials available for
			// local deployments after a database has been reused.
			if u.id == "admin" {
				h, hashErr := hash(u.p)
				if hashErr != nil {
					return hashErr
				}
				if _, err = s.db.Exec("UPDATE users SET password=?,status='启用' WHERE username='admin'", h); err != nil {
					return err
				}
			}
			continue
		}
		h, hashErr := hash(u.p)
		if hashErr != nil {
			return hashErr
		}
		if _, err = s.db.Exec("INSERT INTO users VALUES(?,?,?,?,?,?)", "u-"+u.id, u.id, h, u.n, u.r, "启用"); err != nil {
			return err
		}
	}
	s.db.QueryRow("SELECT COUNT(*) FROM config").Scan(&count)
	if count == 0 {
		b, _ := json.Marshal(defaultConfig())
		_, err = s.db.Exec("INSERT INTO config VALUES(1,?)", b)
	} else {
		// The original default used the full update cycle (7 days) as the
		// warning window. Keep configured values, but migrate that old default
		// to the intended three-day reminder window.
		var raw string
		if s.db.QueryRow("SELECT payload FROM config WHERE id=1").Scan(&raw) == nil {
			var d Dictionary
			if json.Unmarshal([]byte(raw), &d) == nil && d.NearDays == 7 {
				d.NearDays = 3
				if b, marshalErr := json.Marshal(d); marshalErr == nil {
					_, err = s.db.Exec("UPDATE config SET payload=? WHERE id=1", b)
				}
			}
		}
	}
	return err
}
func defaultConfig() Dictionary {
	return Dictionary{Provinces: provinces, ProjectTypes: []string{"政府项目", "国网项目", "科技项目", "其他"}, Stages: []string{"策划中", "申报中", "已立项", "执行中", "已完成", "暂停", "失败", "终止"}, HealthStatuses: []string{"良好", "正常", "有问题", "暂无信息"}, ProblemTags: []string{"进度滞后", "材料缺失", "需求未确认", "协作方未响应", "审批卡点", "合同问题", "预算问题", "其他"}, UpdateCycles: []string{"每周", "每两周", "每月", "自定义"}, NearDays: 3}
}
func (s *Server) routes() {
	s.mux.HandleFunc("GET /api/health", s.health)
	s.mux.HandleFunc("POST /api/login", s.login)
	s.mux.HandleFunc("PUT /api/me/password", s.changePassword)
	s.mux.HandleFunc("GET /api/dictionaries", s.dict)
	s.mux.HandleFunc("GET /api/config", s.config)
	s.mux.HandleFunc("PUT /api/config", s.updateConfig)
	s.mux.HandleFunc("GET /api/users", s.users)
	s.mux.HandleFunc("POST /api/users", s.createUser)
	s.mux.HandleFunc("PUT /api/users/{id}", s.updateUser)
	s.mux.HandleFunc("GET /api/projects", s.list)
	s.mux.HandleFunc("DELETE /api/projects", s.clearProjects)
	s.mux.HandleFunc("POST /api/projects", s.create)
	s.mux.HandleFunc("GET /api/projects/summary", s.summary)
	s.mux.HandleFunc("GET /api/projects/{id}", s.detail)
	s.mux.HandleFunc("DELETE /api/projects/{id}", s.deleteProject)
	s.mux.HandleFunc("PUT /api/projects/{id}", s.edit)
	s.mux.HandleFunc("POST /api/projects/{id}/progress", s.progress)
	// Keep compatibility with clients that used PUT for progress updates.
	s.mux.HandleFunc("PUT /api/projects/{id}/progress", s.progress)
	s.mux.HandleFunc("POST /api/projects/{id}/history/{historyId}/correction", s.correctHistory)
	s.mux.HandleFunc("POST /api/imports/projects", s.importProjects)
	s.mux.HandleFunc("GET /api/exports/projects", s.exportProjects)
	s.mux.HandleFunc("GET /api/exports/history", s.exportHistory)
	s.mux.HandleFunc("/", s.static)
}
func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	jsonOut(w, 200, map[string]string{"status": "ok"})
}
func hash(password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	sum := sha256.Sum256(append(salt, []byte(password)...))
	return hex.EncodeToString(salt) + "$" + hex.EncodeToString(sum[:]), nil
}
func verify(encoded, password string) bool {
	p := strings.Split(encoded, "$")
	if len(p) != 2 {
		return false
	}
	salt, e := hex.DecodeString(p[0])
	if e != nil {
		return false
	}
	sum := sha256.Sum256(append(salt, []byte(password)...))
	want, e := hex.DecodeString(p[1])
	return e == nil && subtle.ConstantTimeCompare(want, sum[:]) == 1
}
func token() string { b := make([]byte, 24); rand.Read(b); return hex.EncodeToString(b) }
func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var q struct{ Username, Password string }
	if json.NewDecoder(r.Body).Decode(&q) != nil {
		fail(w, 400, "请求格式错误")
		return
	}
	var u User
	var encoded string
	if s.db.QueryRow("SELECT id,username,password,name,role,status FROM users WHERE username=?", q.Username).Scan(&u.ID, &u.Username, &encoded, &u.Name, &u.Role, &u.Status) != nil || !verify(encoded, q.Password) || u.Status != "启用" {
		fail(w, 401, "用户名或密码错误")
		return
	}
	t := token()
	s.mu.Lock()
	s.sessions[t] = u
	s.mu.Unlock()
	jsonOut(w, 200, map[string]any{"token": t, "user": u})
}
func (s *Server) auth(r *http.Request, roles ...string) (User, bool) {
	t := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	s.mu.RLock()
	u, ok := s.sessions[t]
	s.mu.RUnlock()
	if !ok {
		return User{}, false
	}
	var status string
	if s.db.QueryRow("SELECT status FROM users WHERE id=?", u.ID).Scan(&status) != nil || status != "启用" {
		return User{}, false
	}
	if len(roles) == 0 {
		return u, true
	}
	for _, role := range roles {
		if u.Role == role {
			return u, true
		}
	}
	return u, false
}

func (s *Server) changePassword(w http.ResponseWriter, r *http.Request) {
	u, ok := s.auth(r)
	if !ok {
		fail(w, 401, "请先登录")
		return
	}
	var req struct {
		CurrentPassword string `json:"currentPassword"`
		NewPassword     string `json:"newPassword"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil || len(req.NewPassword) < 8 {
		fail(w, 400, "新密码至少需要8位")
		return
	}
	var encoded string
	if s.db.QueryRow("SELECT password FROM users WHERE id=?", u.ID).Scan(&encoded) != nil || !verify(encoded, req.CurrentPassword) {
		fail(w, 400, "当前密码不正确")
		return
	}
	hashed, err := hash(req.NewPassword)
	if err != nil {
		fail(w, 500, "密码处理失败")
		return
	}
	if _, err = s.db.Exec("UPDATE users SET password=? WHERE id=?", hashed, u.ID); err != nil {
		fail(w, 500, "密码保存失败")
		return
	}
	jsonOut(w, 200, map[string]string{"status": "saved"})
}
func (s *Server) dict(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.auth(r); !ok {
		fail(w, 401, "请先登录")
		return
	}
	var raw string
	s.db.QueryRow("SELECT payload FROM config WHERE id=1").Scan(&raw)
	var d Dictionary
	json.Unmarshal([]byte(raw), &d)
	known := map[string]bool{}
	for _, p := range d.Provinces {
		known[p] = true
	}
	for _, p := range provinces {
		if !known[p] {
			d.Provinces = append(d.Provinces, p)
		}
	}
	jsonOut(w, 200, d)
}

func (s *Server) config(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.auth(r, "admin"); !ok {
		fail(w, 403, "仅管理员可以访问系统配置")
		return
	}
	var raw string
	if err := s.db.QueryRow("SELECT payload FROM config WHERE id=1").Scan(&raw); err != nil {
		fail(w, 500, "读取配置失败")
		return
	}
	var d Dictionary
	if json.Unmarshal([]byte(raw), &d) != nil {
		fail(w, 500, "配置数据损坏")
		return
	}
	jsonOut(w, 200, d)
}

func (s *Server) updateConfig(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.auth(r, "admin"); !ok {
		fail(w, 403, "仅管理员可以修改系统配置")
		return
	}
	var d Dictionary
	if json.NewDecoder(r.Body).Decode(&d) != nil {
		fail(w, 400, "请求格式错误")
		return
	}
	if len(d.Provinces) == 0 || len(d.ProjectTypes) == 0 || len(d.Stages) == 0 || len(d.HealthStatuses) == 0 || len(d.UpdateCycles) == 0 {
		fail(w, 400, "省份、项目类型、阶段、健康状态和更新周期不能为空")
		return
	}
	if d.NearDays < 1 {
		d.NearDays = 7
	}
	b, _ := json.Marshal(d)
	if _, err := s.db.Exec("UPDATE config SET payload=? WHERE id=1", b); err != nil {
		fail(w, 500, "保存配置失败")
		return
	}
	jsonOut(w, 200, d)
}

func (s *Server) users(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.auth(r); !ok {
		fail(w, 401, "请先登录")
		return
	}
	rows, err := s.db.Query("SELECT id,username,name,role,status FROM users ORDER BY name")
	if err != nil {
		fail(w, 500, "读取用户失败")
		return
	}
	defer rows.Close()
	out := []User{}
	for rows.Next() {
		var u User
		if rows.Scan(&u.ID, &u.Username, &u.Name, &u.Role, &u.Status) == nil {
			out = append(out, u)
		}
	}
	jsonOut(w, 200, out)
}

func (s *Server) createUser(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.auth(r, "admin"); !ok {
		fail(w, 403, "仅管理员可以管理用户")
		return
	}
	var u User
	if json.NewDecoder(r.Body).Decode(&u) != nil || strings.TrimSpace(u.Username) == "" || strings.TrimSpace(u.Name) == "" || len(u.Password) < 8 {
		fail(w, 400, "账号、姓名和至少8位密码不能为空")
		return
	}
	if u.Role != "admin" && u.Role != "user" {
		fail(w, 400, "角色无效")
		return
	}
	if u.Status == "" {
		u.Status = "启用"
	}
	u.ID = "u-" + token()[:12]
	h, err := hash(u.Password)
	if err != nil {
		fail(w, 500, "密码处理失败")
		return
	}
	if _, err = s.db.Exec("INSERT INTO users VALUES(?,?,?,?,?,?)", u.ID, u.Username, h, u.Name, u.Role, u.Status); err != nil {
		fail(w, 409, "账号已存在")
		return
	}
	u.Password = ""
	jsonOut(w, 201, u)
}

func (s *Server) updateUser(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.auth(r, "admin"); !ok {
		fail(w, 403, "仅管理员可以管理用户")
		return
	}
	var in User
	if json.NewDecoder(r.Body).Decode(&in) != nil {
		fail(w, 400, "请求格式错误")
		return
	}
	if in.Role != "" && in.Role != "admin" && in.Role != "user" {
		fail(w, 400, "角色无效")
		return
	}
	if in.Status != "" && in.Status != "启用" && in.Status != "停用" {
		fail(w, 400, "状态无效")
		return
	}
	if in.ID == "" {
		in.ID = r.PathValue("id")
	}
	if in.ID == "u-admin" && in.Status == "停用" {
		fail(w, 400, "不能停用唯一管理员")
		return
	}
	password := ""
	if in.Password != "" {
		if len(in.Password) < 8 {
			fail(w, 400, "密码至少需要8位")
			return
		}
		password, _ = hash(in.Password)
	}
	_, err := s.db.Exec("UPDATE users SET name=COALESCE(NULLIF(?,''),name),role=COALESCE(NULLIF(?,''),role),status=COALESCE(NULLIF(?,''),status),password=COALESCE(NULLIF(?,''),password) WHERE id=?", in.Name, in.Role, in.Status, password, r.PathValue("id"))
	if err != nil {
		fail(w, 500, "用户保存失败")
		return
	}
	jsonOut(w, 200, map[string]string{"status": "saved"})
}
func normalize(p *Project) {
	if p.ProjectYear == "" {
		p.ProjectYear = strconv.Itoa(time.Now().Year())
	}
	if p.Stage == "" {
		p.Stage = "策划中"
	}
	if p.Health == "" {
		p.Health = "暂无信息"
	}
	if p.UpdateCycle == "" {
		p.UpdateCycle = "每周"
	}
	if p.UpdatedAt == "" {
		p.UpdatedAt = time.Now().Format(time.RFC3339)
	}
	setNextUpdateAt(p)
}

func reminderEnabled(stage string) bool {
	return stage == "策划中" || stage == "申报中"
}

func setNextUpdateAt(p *Project) {
	if !reminderEnabled(p.Stage) {
		p.NextUpdateAt = ""
		return
	}
	p.NextUpdateAt = nextDate(p.UpdatedAt, p.UpdateCycle, p.CustomCycleDays)
}
func nextDate(at, cycle string, custom int) string {
	t, e := time.Parse(time.RFC3339, at)
	if e != nil {
		t = time.Now()
	}
	days := 7
	if cycle == "每两周" {
		days = 14
	}
	if cycle == "每月" {
		days = 30
	}
	if cycle == "自定义" && custom > 0 {
		days = custom
	}
	return t.AddDate(0, 0, days).Format(time.RFC3339)
}
func valid(p Project) error {
	if strings.TrimSpace(p.Province) == "" || strings.TrimSpace(p.Name) == "" || strings.TrimSpace(p.ProjectYear) == "" {
		return errors.New("省份、项目名称、项目年份不能为空")
	}
	if p.Health == "有问题" && strings.TrimSpace(p.ProblemDescription) == "" {
		return errors.New("健康状态为有问题时必须填写问题说明")
	}
	if p.UpdateCycle == "自定义" && p.CustomCycleDays < 1 {
		return errors.New("自定义更新周期必须大于0天")
	}
	return nil
}
func (s *Server) conflict(p Project, id string) bool {
	rows, err := s.db.Query("SELECT id,project_no FROM projects WHERE province=? AND name=? AND project_year=? AND id<>?", p.Province, p.Name, p.ProjectYear, id)
	if err != nil {
		return true
	}
	defer rows.Close()
	for rows.Next() {
		var found, projectNo string
		if rows.Scan(&found, &projectNo) != nil {
			return true
		}
		if strings.TrimSpace(p.ProjectNo) == "" || strings.TrimSpace(projectNo) == "" || p.ProjectNo == projectNo {
			return true
		}
	}
	return false
}
func (s *Server) create(w http.ResponseWriter, r *http.Request) {
	u, ok := s.auth(r, "admin", "user")
	if !ok {
		fail(w, 403, "无权新增项目")
		return
	}
	var in projectInput
	if json.NewDecoder(r.Body).Decode(&in) != nil {
		fail(w, 400, "请求格式错误")
		return
	}
	normalize(&in.Project)
	if e := valid(in.Project); e != nil {
		fail(w, 400, e.Error())
		return
	}
	if s.conflict(in.Project, "") {
		fail(w, 409, "同省份、同名称、同年份项目已存在")
		return
	}
	p := in.Project
	p.ID = "p-" + token()[:12]
	p.UpdatedBy = u.Name
	p.CreatedAt = p.UpdatedAt
	p.CreatedBy = u.Name
	p.Version = 1
	b, _ := json.Marshal(p)
	tx, _ := s.db.Begin()
	if _, e := tx.Exec("INSERT INTO projects VALUES(?,?,?,?,?,?,?,?,?)", p.ID, p.Province, p.Name, p.ProjectYear, p.ProjectNo, b, p.UpdatedAt, p.NextUpdateAt, p.Version); e != nil {
		tx.Rollback()
		fail(w, 409, "项目已存在")
		return
	}
	if e := insertHistory(tx, p, in.Note); e != nil {
		tx.Rollback()
		fail(w, 500, "历史记录保存失败")
		return
	}
	tx.Commit()
	s.audit(p.ID, u.Name, map[string]any{"type": "project_create", "after": p})
	jsonOut(w, 201, p)
}

func (s *Server) clearProjects(w http.ResponseWriter, r *http.Request) {
	u, ok := s.auth(r, "admin")
	if !ok {
		fail(w, 403, "仅管理员可以清空项目")
		return
	}
	tx, err := s.db.Begin()
	if err != nil {
		fail(w, 500, "无法开始清空操作")
		return
	}
	var count int
	if err = tx.QueryRow("SELECT COUNT(*) FROM projects").Scan(&count); err == nil {
		_, err = tx.Exec("DELETE FROM histories; DELETE FROM audit_logs; DELETE FROM import_batches; DELETE FROM projects;")
	}
	if err != nil {
		tx.Rollback()
		fail(w, 500, "项目清空失败")
		return
	}
	if err = tx.Commit(); err != nil {
		fail(w, 500, "项目清空失败")
		return
	}
	jsonOut(w, 200, map[string]any{"status": "cleared", "count": count, "operator": u.Name})
}

func (s *Server) deleteProject(w http.ResponseWriter, r *http.Request) {
	u, ok := s.auth(r, "admin")
	if !ok {
		fail(w, 403, "仅管理员可以删除项目")
		return
	}
	id := r.PathValue("id")
	tx, err := s.db.Begin()
	if err != nil {
		fail(w, 500, "无法开始删除操作")
		return
	}
	var count int
	if err = tx.QueryRow("SELECT COUNT(*) FROM projects WHERE id=?", id).Scan(&count); err != nil || count == 0 {
		tx.Rollback()
		fail(w, 404, "项目不存在")
		return
	}
	if _, err = tx.Exec("DELETE FROM histories WHERE project_id=?", id); err == nil {
		_, err = tx.Exec("DELETE FROM audit_logs WHERE project_id=?", id)
	}
	if err == nil {
		_, err = tx.Exec("DELETE FROM projects WHERE id=?", id)
	}
	if err != nil {
		tx.Rollback()
		fail(w, 500, "项目删除失败")
		return
	}
	if err = tx.Commit(); err != nil {
		fail(w, 500, "项目删除失败")
		return
	}
	jsonOut(w, 200, map[string]any{"status": "deleted", "id": id, "operator": u.Name})
}

func insertHistory(tx *sql.Tx, p Project, note string) error {
	h := History{ProjectID: p.ID, Stage: p.Stage, Health: p.Health, Progress: p.Progress, ProblemTags: p.ProblemTags, ProblemDescription: p.ProblemDescription, NextPlan: p.NextPlan, Note: note, UpdatedAt: p.UpdatedAt, UpdatedBy: p.UpdatedBy}
	b, _ := json.Marshal(h)
	_, e := tx.Exec("INSERT INTO histories(project_id,payload,updated_at) VALUES(?,?,?)", p.ID, b, p.UpdatedAt)
	return e
}
func (s *Server) get(id string) (Project, error) {
	var b []byte
	var p Project
	e := s.db.QueryRow("SELECT payload FROM projects WHERE id=?", id).Scan(&b)
	if e == nil {
		e = json.Unmarshal(b, &p)
	}
	return p, e
}

func appendFilter(where *[]string, args *[]any, column, raw string) {
	values := make([]string, 0)
	for _, value := range strings.Split(raw, ",") {
		if value = strings.TrimSpace(value); value != "" {
			if column == "province" && value == "__all_provinces__" {
				for _, province := range provinces { if !contains(directUnits, province) { values = append(values, province) } }
				continue
			}
			if column == "province" && value == "__all_direct_units__" { values = append(values, directUnits...); continue }
			values = append(values, value)
		}
	}
	if len(values) == 0 {
		return
	}
	if len(values) == 1 {
		*where = append(*where, column+"=?")
		*args = append(*args, values[0])
		return
	}
	placeholders := strings.TrimRight(strings.Repeat("?,", len(values)), ",")
	*where = append(*where, column+" IN ("+placeholders+")")
	for _, value := range values {
		*args = append(*args, value)
	}
}

func contains(items []string, target string) bool { for _, item := range items { if item == target { return true } }; return false }
func (s *Server) list(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.auth(r); !ok {
		fail(w, 401, "请先登录")
		return
	}
	q := r.URL.Query()
	where := []string{"1=1"}
	args := []any{}
	for _, f := range []struct{ k, c string }{{"province", "province"}, {"stage", "json_extract(payload,'$.stage')"}, {"health", "json_extract(payload,'$.health')"}, {"type", "json_extract(payload,'$.type')"}, {"specificType", "json_extract(payload,'$.specificType')"}, {"expansionOwner", "json_extract(payload,'$.expansionOwner')"}, {"projectYear", "project_year"}} {
		if v := q.Get(f.k); v != "" {
			appendFilter(&where, &args, f.c, v)
		}
	}
	if v := strings.TrimSpace(q.Get("q")); v != "" {
		where = append(where, "(name LIKE ? OR province LIKE ? OR project_no LIKE ?)")
		args = append(args, "%"+v+"%", "%"+v+"%", "%"+v+"%")
	}
	if v := q.Get("successDateFrom"); v != "" {
		where = append(where, "json_extract(payload,'$.successDate') >= ?")
		args = append(args, v)
	}
	if v := q.Get("successDateTo"); v != "" {
		where = append(where, "json_extract(payload,'$.successDate') <= ?")
		args = append(args, v)
	}
	nearDays := s.nearDays()
	if quick := q.Get("quick"); quick != "" {
		switch quick {
		case "problem":
			where = append(where, "json_extract(payload,'$.health')='有问题'")
		case "overdue":
			where = append(where, "json_extract(payload,'$.stage') IN ('策划中','申报中') AND next_update_at < ?")
			args = append(args, time.Now().Format(time.RFC3339))
		case "soon":
			where = append(where, "json_extract(payload,'$.stage') IN ('策划中','申报中') AND next_update_at >= ? AND next_update_at < ?")
			args = append(args, time.Now().Format(time.RFC3339), time.Now().AddDate(0, 0, nearDays).Format(time.RFC3339))
		case "good", "normal":
			where = append(where, "json_extract(payload,'$.health')=?")
			args = append(args, map[string]string{"good": "良好", "normal": "正常"}[quick])
		}
	}
	var total int
	s.db.QueryRow("SELECT COUNT(*) FROM projects WHERE "+strings.Join(where, " AND "), args...).Scan(&total)
	page, _ := strconv.Atoi(q.Get("page"))
	if page < 1 {
		page = 1
	}
	size, _ := strconv.Atoi(q.Get("pageSize"))
	if size < 1 || size > 1000 {
		size = 20
	}
	// The SQL clause is LIMIT ? OFFSET ?, so the row count comes first.
	args = append(args, size, (page-1)*size)
	order := "updated_at DESC"
	switch q.Get("sort") {
	case "name":
		order = "name COLLATE NOCASE ASC"
	case "stage":
		order = "json_extract(payload,'$.stage') ASC, updated_at DESC"
	case "health":
		order = "json_extract(payload,'$.health') ASC, updated_at DESC"
	case "province":
		order = "province ASC, name COLLATE NOCASE ASC"
	}
	rows, e := s.db.Query("SELECT payload FROM projects WHERE "+strings.Join(where, " AND ")+" ORDER BY "+order+" LIMIT ? OFFSET ?", args...)
	if e != nil {
		fail(w, 500, "项目读取失败")
		return
	}
	defer rows.Close()
	// Payloads are already validated JSON at write time. Return them directly
	// so a legacy field type cannot silently remove a row from the ledger.
	items := []json.RawMessage{}
	for rows.Next() {
		var b []byte
		if rows.Scan(&b) == nil && json.Valid(b) {
			items = append(items, json.RawMessage(b))
		}
	}
	if err := rows.Err(); err != nil {
		fail(w, 500, "项目读取失败")
		return
	}
	jsonOut(w, 200, map[string]any{"items": items, "total": total, "page": page, "pageSize": size})
}

func (s *Server) nearDays() int {
	var raw string
	if s.db.QueryRow("SELECT payload FROM config WHERE id=1").Scan(&raw) != nil {
		return 3
	}
	var d Dictionary
	if json.Unmarshal([]byte(raw), &d) != nil || d.NearDays < 1 {
		return 3
	}
	return d.NearDays
}
func (s *Server) detail(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.auth(r); !ok {
		fail(w, 401, "请先登录")
		return
	}
	p, e := s.get(r.PathValue("id"))
	if e != nil {
		fail(w, 404, "项目不存在")
		return
	}
	rows, _ := s.db.Query("SELECT id,payload FROM histories WHERE project_id=? ORDER BY id DESC", p.ID)
	defer rows.Close()
	hs := []History{}
	for rows.Next() {
		var id int64
		var b string
		var h History
		if rows.Scan(&id, &b) == nil && json.Unmarshal([]byte(b), &h) == nil {
			h.ID = id
			hs = append(hs, h)
		}
	}
	jsonOut(w, 200, map[string]any{"project": p, "history": hs})
}
func (s *Server) edit(w http.ResponseWriter, r *http.Request) {
	u, ok := s.auth(r, "admin", "user")
	if !ok {
		fail(w, 403, "无权编辑项目")
		return
	}
	var in projectInput
	if json.NewDecoder(r.Body).Decode(&in) != nil {
		fail(w, 400, "请求格式错误")
		return
	}
	normalize(&in.Project)
	old, e := s.get(r.PathValue("id"))
	if e != nil {
		fail(w, 404, "项目不存在")
		return
	}
	if in.Version != 0 && in.Version != old.Version {
		fail(w, 409, "项目已被更新，请刷新后重试")
		return
	}
	if e = valid(in.Project); e != nil {
		fail(w, 400, e.Error())
		return
	}
	if s.conflict(in.Project, old.ID) {
		fail(w, 409, "同省份、同名称、同年份项目已存在")
		return
	}
	p := old
	p.Province = in.Province
	p.Name = in.Name
	p.Source = in.Source
	p.Type = in.Type
	p.SpecificType = in.SpecificType
	p.ProjectYear = in.ProjectYear
	p.ResponsibleDepartment = in.ResponsibleDepartment
	p.ExpansionOwner = in.ExpansionOwner
	p.LeadParticipatingUnits = in.LeadParticipatingUnits
	p.TotalAmount = in.TotalAmount
	p.ContractAmount = in.ContractAmount
	p.SuccessDate = in.SuccessDate
	p.ZhixinRole = in.ZhixinRole
	p.InternalSupportDepartment = in.InternalSupportDepartment
	p.ProvincialSupportDepartment = in.ProvincialSupportDepartment
	p.GovernmentUnit = in.GovernmentUnit
	p.UpdatedAt = time.Now().Format(time.RFC3339)
	p.NextUpdateAt = old.NextUpdateAt
	p.UpdatedBy = u.Name
	p.CreatedAt = old.CreatedAt
	p.CreatedBy = old.CreatedBy
	p.Version = old.Version + 1
	b, _ := json.Marshal(p)
	_, e = s.db.Exec("UPDATE projects SET province=?,name=?,project_year=?,project_no=?,payload=?,updated_at=?,next_update_at=?,version=? WHERE id=? AND version=?", p.Province, p.Name, p.ProjectYear, p.ProjectNo, b, p.UpdatedAt, p.NextUpdateAt, p.Version, p.ID, old.Version)
	if e != nil {
		fail(w, 500, "项目保存失败")
		return
	}
	s.audit(p.ID, u.Name, map[string]any{"type": "project_edit", "before": old, "after": p})
	jsonOut(w, 200, p)
}

func (s *Server) audit(projectID, operator string, value any) {
	b, _ := json.Marshal(value)
	_, _ = s.db.Exec("INSERT INTO audit_logs(project_id,operator,changed_at,payload) VALUES(?,?,?,?)", projectID, operator, time.Now().Format(time.RFC3339), b)
}
func (s *Server) progress(w http.ResponseWriter, r *http.Request) {
	log.Printf("project progress: received id=%s", r.PathValue("id"))
	u, ok := s.auth(r, "admin", "user")
	if !ok {
		fail(w, 403, "无权更新进展")
		return
	}
	var in projectInput
	if json.NewDecoder(r.Body).Decode(&in) != nil {
		log.Printf("project progress: invalid request id=%s", r.PathValue("id"))
		fail(w, 400, "请求格式错误")
		return
	}
	log.Printf("project progress: payload id=%s health=%q progress_len=%d tags=%d version=%d", r.PathValue("id"), in.Health, len(in.Progress), len(in.ProblemTags), in.Version)
	old, e := s.get(r.PathValue("id"))
	if e != nil {
		fail(w, 404, "项目不存在")
		return
	}
	normalize(&in.Project)
	p := old
	p.Stage = in.Stage
	p.Health = in.Health
	p.Progress = in.Progress
	p.ProblemTags = in.ProblemTags
	p.ProblemDescription = in.ProblemDescription
	p.NextPlan = in.NextPlan
	p.UpdateCycle = in.UpdateCycle
	p.CustomCycleDays = in.CustomCycleDays
	p.TotalAmount = in.TotalAmount
	p.ContractAmount = in.ContractAmount
	p.SuccessDate = in.SuccessDate
	p.UpdatedAt = time.Now().Format(time.RFC3339)
	setNextUpdateAt(&p)
	p.UpdatedBy = u.Name
	p.Version = old.Version + 1
	if e = valid(p); e != nil {
		log.Printf("project progress: validation failed id=%s health=%q error=%v", p.ID, p.Health, e)
		fail(w, 400, e.Error())
		return
	}
	b, _ := json.Marshal(p)
	tx, e := s.db.Begin()
	if e != nil {
		fail(w, 500, "进展保存失败")
		return
	}
	var result sql.Result
	if result, e = tx.Exec("UPDATE projects SET payload=?,updated_at=?,next_update_at=?,version=? WHERE id=? AND version=?", b, p.UpdatedAt, p.NextUpdateAt, p.Version, p.ID, old.Version); e != nil {
		log.Printf("project progress: update failed id=%s error=%v", p.ID, e)
		tx.Rollback()
		fail(w, 409, "项目已被更新，请刷新后重试")
		return
	}
	if affected, rowsErr := result.RowsAffected(); rowsErr != nil || affected != 1 {
		log.Printf("project progress: stale update id=%s affected=%d error=%v", p.ID, affected, rowsErr)
		tx.Rollback()
		fail(w, 409, "项目已被更新，请刷新后重试")
		return
	}
	if e = insertHistory(tx, p, in.Note); e != nil {
		log.Printf("project progress: history failed id=%s error=%v", p.ID, e)
		tx.Rollback()
		fail(w, 500, "历史记录保存失败")
		return
	}
	if e = tx.Commit(); e != nil {
		log.Printf("project progress: commit failed id=%s error=%v", p.ID, e)
		fail(w, 500, "进展保存失败")
		return
	}
	log.Printf("project progress: saved id=%s health=%q version=%d", p.ID, p.Health, p.Version)
	s.audit(p.ID, u.Name, map[string]any{"type": "progress_update", "before": old, "after": p})
	jsonOut(w, 200, p)
}

func (s *Server) correctHistory(w http.ResponseWriter, r *http.Request) {
	u, ok := s.auth(r, "admin", "user")
	if !ok {
		fail(w, 403, "无权修正历史记录")
		return
	}
	project, err := s.get(r.PathValue("id"))
	if err != nil {
		fail(w, 404, "项目不存在")
		return
	}
	originalID, err := strconv.ParseInt(r.PathValue("historyId"), 10, 64)
	if err != nil {
		fail(w, 400, "历史记录编号无效")
		return
	}
	var original History
	var raw string
	if err = s.db.QueryRow("SELECT payload FROM histories WHERE id=? AND project_id=?", originalID, project.ID).Scan(&raw); err != nil || json.Unmarshal([]byte(raw), &original) != nil {
		fail(w, 404, "历史记录不存在")
		return
	}
	var req struct {
		Progress           string `json:"progress"`
		ProblemDescription string `json:"problemDescription"`
		NextPlan           string `json:"nextPlan"`
		Note               string `json:"note"`
		CorrectionNote     string `json:"correctionNote"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil {
		fail(w, 400, "请求格式错误")
		return
	}
	if strings.TrimSpace(req.CorrectionNote) == "" {
		fail(w, 400, "修正说明不能为空")
		return
	}
	h := original
	h.ID = 0
	h.ProjectID = project.ID
	h.Progress = req.Progress
	h.ProblemDescription = req.ProblemDescription
	h.NextPlan = req.NextPlan
	h.Note = req.Note
	h.UpdatedAt = time.Now().Format(time.RFC3339)
	h.UpdatedBy = u.Name
	h.CorrectionOf = originalID
	h.CorrectionNote = req.CorrectionNote
	tx, err := s.db.Begin()
	if err != nil {
		fail(w, 500, "历史修正保存失败")
		return
	}
	if err = insertHistoryRecord(tx, h); err != nil {
		tx.Rollback()
		fail(w, 500, "历史修正保存失败")
		return
	}
	if err = tx.Commit(); err != nil {
		fail(w, 500, "历史修正保存失败")
		return
	}
	jsonOut(w, 201, h)
}

func insertHistoryRecord(tx *sql.Tx, h History) error {
	b, _ := json.Marshal(h)
	_, err := tx.Exec("INSERT INTO histories(project_id,payload,updated_at) VALUES(?,?,?)", h.ProjectID, b, h.UpdatedAt)
	return err
}
func (s *Server) summary(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.auth(r); !ok {
		fail(w, 401, "请先登录")
		return
	}
	rows, _ := s.db.Query("SELECT province,COUNT(*),SUM(CASE WHEN json_extract(payload,'$.health')='有问题' THEN 1 ELSE 0 END) FROM projects GROUP BY province ORDER BY province")
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var p string
		var n, b int
		rows.Scan(&p, &n, &b)
		out = append(out, map[string]any{"province": p, "total": n, "issues": b})
	}
	jsonOut(w, 200, out)
}

func (s *Server) exportProjects(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.auth(r); !ok {
		fail(w, 401, "请先登录")
		return
	}
	q := r.URL.Query()
	where := []string{"1=1"}
	args := []any{}
	for _, f := range []struct{ k, c string }{{"province", "province"}, {"stage", "json_extract(payload,'$.stage')"}, {"health", "json_extract(payload,'$.health')"}, {"type", "json_extract(payload,'$.type')"}, {"specificType", "json_extract(payload,'$.specificType')"}, {"expansionOwner", "json_extract(payload,'$.expansionOwner')"}, {"projectYear", "project_year"}} {
		if v := q.Get(f.k); v != "" {
			appendFilter(&where, &args, f.c, v)
		}
	}
	if v := strings.TrimSpace(q.Get("q")); v != "" {
		where = append(where, "(name LIKE ? OR province LIKE ? OR project_no LIKE ?)")
		args = append(args, "%"+v+"%", "%"+v+"%", "%"+v+"%")
	}
	if v := q.Get("successDateFrom"); v != "" {
		where = append(where, "json_extract(payload,'$.successDate') >= ?")
		args = append(args, v)
	}
	if v := q.Get("successDateTo"); v != "" {
		where = append(where, "json_extract(payload,'$.successDate') <= ?")
		args = append(args, v)
	}
	if quick := q.Get("quick"); quick == "overdue" {
		where = append(where, "json_extract(payload,'$.stage') IN ('策划中','申报中') AND next_update_at < ?")
		args = append(args, time.Now().Format(time.RFC3339))
	} else if quick == "soon" {
		where = append(where, "json_extract(payload,'$.stage') IN ('策划中','申报中') AND next_update_at >= ? AND next_update_at < ?")
		args = append(args, time.Now().Format(time.RFC3339), time.Now().AddDate(0, 0, s.nearDays()).Format(time.RFC3339))
	} else if quick == "problem" || quick == "good" || quick == "normal" {
		health := map[string]string{"problem": "有问题", "good": "良好", "normal": "正常"}[quick]
		where = append(where, "json_extract(payload,'$.health')=?")
		args = append(args, health)
	}
	rows, err := s.db.Query("SELECT payload FROM projects WHERE "+strings.Join(where, " AND ")+" ORDER BY province,name", args...)
	if err != nil {
		fail(w, 500, "导出失败")
		return
	}
	defer rows.Close()
	out := []Project{}
	for rows.Next() {
		var b []byte
		var p Project
		if rows.Scan(&b) == nil && json.Unmarshal(b, &p) == nil {
			out = append(out, p)
		}
	}
	jsonOut(w, 200, out)
}

func (s *Server) exportHistory(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.auth(r); !ok {
		fail(w, 401, "请先登录")
		return
	}
	projectID := r.URL.Query().Get("projectId")
	query := "SELECT id,project_id,payload FROM histories"
	args := []any{}
	if projectID != "" {
		query += " WHERE project_id=?"
		args = append(args, projectID)
	}
	query += " ORDER BY id DESC"
	rows, err := s.db.Query(query, args...)
	if err != nil {
		fail(w, 500, "历史导出失败")
		return
	}
	defer rows.Close()
	out := []History{}
	for rows.Next() {
		var id int64
		var projectID, raw string
		var h History
		if rows.Scan(&id, &projectID, &raw) == nil && json.Unmarshal([]byte(raw), &h) == nil {
			h.ID = id
			h.ProjectID = projectID
			out = append(out, h)
		}
	}
	jsonOut(w, 200, out)
}

func (s *Server) importProjects(w http.ResponseWriter, r *http.Request) {
	u, ok := s.auth(r, "admin", "user")
	if !ok {
		fail(w, 403, "无权导入项目")
		return
	}
	var req struct {
		Projects []Project `json:"projects"`
		Confirm  bool      `json:"confirm"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil {
		fail(w, 400, "请求格式错误")
		return
	}
	result := map[string]any{"created": 0, "updated": 0, "errors": []any{}}
	batchID := "import-" + token()[:12]
	startedAt := time.Now().Format(time.RFC3339)
	if req.Confirm {
		b, _ := json.Marshal(map[string]any{"rows": len(req.Projects), "created": 0, "updated": 0})
		_, _ = s.db.Exec("INSERT INTO import_batches(id,operator,started_at,payload) VALUES(?,?,?,?)", batchID, u.Name, startedAt, b)
	}
	for i, p := range req.Projects {
		normalize(&p)
		if e := valid(p); e != nil {
			result["errors"] = append(result["errors"].([]any), map[string]any{"row": i + 2, "message": e.Error()})
			continue
		}
		existingID := ""
		rows, _ := s.db.Query("SELECT id,project_no FROM projects WHERE province=? AND name=? AND project_year=?", p.Province, p.Name, p.ProjectYear)
		for rows != nil && rows.Next() {
			var id, no string
			rows.Scan(&id, &no)
			if p.ProjectNo == "" || no == "" || p.ProjectNo == no {
				existingID = id
				break
			}
		}
		if rows != nil {
			rows.Close()
		}
		if !req.Confirm {
			if existingID == "" {
				result["created"] = result["created"].(int) + 1
			} else {
				result["updated"] = result["updated"].(int) + 1
			}
			continue
		}
		p.UpdatedAt = time.Now().Format(time.RFC3339)
		p.UpdatedBy = u.Name
		setNextUpdateAt(&p)
		b, _ := json.Marshal(p)
		if existingID == "" {
			p.ID = "p-" + token()[:12]
			p.Version = 1
			p.CreatedAt = p.UpdatedAt
			p.CreatedBy = u.Name
			b, _ = json.Marshal(p)
			tx, _ := s.db.Begin()
			_, e := tx.Exec("INSERT INTO projects VALUES(?,?,?,?,?,?,?,?,?)", p.ID, p.Province, p.Name, p.ProjectYear, p.ProjectNo, b, p.UpdatedAt, p.NextUpdateAt, p.Version)
			if e == nil {
				e = insertHistory(tx, p, "Excel 导入")
			}
			if e == nil {
				e = tx.Commit()
			} else {
				tx.Rollback()
			}
			if e == nil {
				result["created"] = result["created"].(int) + 1
			}
		} else {
			old, e := s.get(existingID)
			if e == nil {
				p.ID = existingID
				p.CreatedAt = old.CreatedAt
				p.CreatedBy = old.CreatedBy
				p.Version = old.Version + 1
				b, _ = json.Marshal(p)
				tx, txErr := s.db.Begin()
				if txErr == nil {
					_, txErr = tx.Exec("UPDATE projects SET payload=?,updated_at=?,next_update_at=?,version=? WHERE id=? AND version=?", b, p.UpdatedAt, p.NextUpdateAt, p.Version, p.ID, old.Version)
					if txErr == nil {
						txErr = insertHistory(tx, p, "Excel 导入")
					}
					if txErr == nil {
						txErr = tx.Commit()
					} else {
						tx.Rollback()
					}
				}
				if txErr == nil {
					s.audit(p.ID, u.Name, map[string]any{"type": "excel_update", "before": old, "after": p})
					result["updated"] = result["updated"].(int) + 1
				}
			}
		}
	}
	if req.Confirm {
		b, _ := json.Marshal(result)
		_, _ = s.db.Exec("UPDATE import_batches SET completed_at=?,payload=? WHERE id=?", time.Now().Format(time.RFC3339), b, batchID)
	}
	jsonOut(w, 200, result)
}

func (s *Server) insertHistoryDirect(p Project, note string) error {
	tx, e := s.db.Begin()
	if e != nil {
		return e
	}
	if e = insertHistory(tx, p, note); e != nil {
		tx.Rollback()
		return e
	}
	return tx.Commit()
}
func (s *Server) static(w http.ResponseWriter, r *http.Request) {
	path := filepath.Join(s.frontend, r.URL.Path)
	if r.URL.Path == "/" {
		path = filepath.Join(s.frontend, "index.html")
	}
	if _, e := os.Stat(path); e != nil {
		path = filepath.Join(s.frontend, "index.html")
	}
	http.ServeFile(w, r, path)
}
func jsonOut(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]any{"data": v})
}
func fail(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		if r.Method == "OPTIONS" {
			w.WriteHeader(204)
			return
		}
		next.ServeHTTP(w, r)
	})
}
