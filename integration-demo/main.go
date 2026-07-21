package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

const providerCode = "livacloud-demo"

type configuration struct {
	InboundSecret  string `json:"inbound_secret"`
	OutboundSecret string `json:"outbound_secret"`
}

type order struct {
	ID           string    `json:"id"`
	RequestID    string    `json:"request_id"`
	FileName     string    `json:"file_name"`
	Status       string    `json:"status"`
	ErrorCode    string    `json:"error_code,omitempty"`
	ErrorMessage string    `json:"error_message,omitempty"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type persistedState struct {
	Configuration configuration    `json:"configuration"`
	Orders        map[string]order `json:"orders"`
	Events        map[string]bool  `json:"events"`
}

type server struct {
	mu                                sync.RWMutex
	state                             persistedState
	dataDir, cloudBase, adminPassword string
}

func main() {
	s := &server{dataDir: env("DEMO_DATA_DIR", "/data"), cloudBase: strings.TrimRight(env("DEMO_CLOUD_API_BASE", "http://api:8080"), "/"), adminPassword: env("DEMO_ADMIN_PASSWORD", "demo123")}
	if err := os.MkdirAll(filepath.Join(s.dataDir, "files"), 0700); err != nil {
		log.Fatal(err)
	}
	s.load()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("GET /entry", s.entry)
	mux.HandleFunc("GET /mock-sso", s.mockSSO)
	mux.HandleFunc("GET /sso/callback", s.ssoCallback)
	mux.HandleFunc("GET /documents", s.documents)
	mux.HandleFunc("GET /setup", s.setupPage)
	mux.HandleFunc("GET /api/setup/status", s.setupStatus)
	mux.HandleFunc("POST /api/setup", s.saveSetup)
	mux.HandleFunc("POST /api/orders", s.createOrder)
	mux.HandleFunc("GET /api/orders/{id}", s.getOrder)
	mux.HandleFunc("GET /files/{id}", s.serveFile)
	mux.HandleFunc("POST /api/print/callback", s.callback)
	go s.cleanupFiles()
	log.Println("丽娃云聘 Demo listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", securityHeaders(mux)))
}

func (s *server) load() {
	s.state = persistedState{Orders: map[string]order{}, Events: map[string]bool{}}
	raw, err := os.ReadFile(filepath.Join(s.dataDir, "state.json"))
	if err != nil {
		return
	}
	if json.Unmarshal(raw, &s.state) != nil {
		log.Println("忽略无法解析的 Demo 状态文件")
	}
	if s.state.Orders == nil {
		s.state.Orders = map[string]order{}
	}
	if s.state.Events == nil {
		s.state.Events = map[string]bool{}
	}
}

func (s *server) saveLocked() error {
	raw, err := json.MarshalIndent(s.state, "", "  ")
	if err != nil {
		return err
	}
	tmp := filepath.Join(s.dataDir, "state.json.tmp")
	if err = os.WriteFile(tmp, raw, 0600); err != nil {
		return err
	}
	return os.Rename(tmp, filepath.Join(s.dataDir, "state.json"))
}

func (s *server) configured() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state.Configuration.InboundSecret != "" && s.state.Configuration.OutboundSecret != ""
}

func (s *server) entry(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Get("terminal_ticket") == "" {
		renderError(w, "终端票据无效", "请返回飞印终端重新扫码。", http.StatusBadRequest)
		return
	}
	renderHTML(w, page("丽娃云聘 Demo", `<h1>丽娃云聘 Demo</h1><p>这是用于验证飞印第三方接入协议的模拟系统。</p><button id="login">模拟 SSO 登录</button><script>sessionStorage.setItem('flyprint_terminal_ticket',new URLSearchParams(location.search).get('terminal_ticket'));document.getElementById('login').onclick=()=>location.href='/integration-demo/mock-sso'</script>`))
}

func (s *server) mockSSO(w http.ResponseWriter, _ *http.Request) {
	renderHTML(w, page("模拟统一认证", `<h1>统一身份认证</h1><p>测试用户：张老师（工号 DEMO001）</p><button onclick="location.href='/integration-demo/sso/callback'">确认登录</button>`))
}
func (s *server) ssoCallback(w http.ResponseWriter, _ *http.Request) {
	renderHTML(w, page("登录完成", `<h1>登录成功</h1><p>正在返回丽娃云聘……</p><script>if(!sessionStorage.getItem('flyprint_terminal_ticket')){document.body.innerHTML='<main><h1>终端票据已丢失</h1><p>请返回终端重新扫码。</p></main>'}else{location.replace('/integration-demo/documents')}</script>`))
}

func (s *server) documents(w http.ResponseWriter, _ *http.Request) {
	body := `<h1>材料打印</h1><p class="muted">当前用户：张老师（DEMO001）</p><label>选择 PDF（不选择则使用内置测试页）</label><input id="file" type="file" accept="application/pdf"><button id="submit">提交到当前飞印终端</button><div id="status" class="status"></div><script>
const ticket=sessionStorage.getItem('flyprint_terminal_ticket');if(!ticket){location.replace('/integration-demo/entry')}
const status=document.getElementById('status');document.getElementById('submit').onclick=async()=>{const b=document.getElementById('submit');b.disabled=true;status.textContent='正在提交…';const f=new FormData();f.append('terminal_ticket',ticket);const file=document.getElementById('file').files[0];if(file)f.append('file',file);try{const r=await fetch('/integration-demo/api/orders',{method:'POST',body:f});const d=await r.json();if(!r.ok)throw new Error(d.message||'提交失败');status.textContent='已提交，请回到飞印终端预览并确认。';poll(d.order_id)}catch(e){status.textContent=e.message;b.disabled=false}}
async function poll(id){const r=await fetch('/integration-demo/api/orders/'+id,{cache:'no-store'});const d=await r.json();const names={waiting_file:'正在接收文件',waiting_terminal:'等待终端确认',dispatched:'任务已下发',printing:'正在打印',completed:'打印完成',failed:'打印失败',expired:'操作已过期',cancelled:'任务已取消'};status.textContent=names[d.status]||d.status;if(!['completed','failed','expired','cancelled'].includes(d.status))setTimeout(()=>poll(id),1000)}
</script>`
	renderHTML(w, page("材料打印", body))
}

func (s *server) setupPage(w http.ResponseWriter, r *http.Request) {
	templateValues := fmt.Sprintf(`{"code":"%s","display_name":"丽娃云聘 Demo","entry_url":"%s/integration-demo/entry","callback_base_url":"http://integration-demo:8080","allowed_ip_cidrs":"172.16.0.0/12","allowed_file_hosts":"integration-demo","allow_private_file_hosts":true,"max_file_size":10485760,"allowed_mime_types":"application/pdf"}`, providerCode, schemeHost(r))
	body := `<h1>Demo 接入设置</h1><p>先在飞印管理端创建 provider，再将一次性密钥粘贴到这里。</p><label>Demo 管理密码</label><input id="password" type="password"><label>入站密钥（Demo → FlyPrint）</label><input id="inbound" type="password"><label>出站密钥（FlyPrint → Demo）</label><input id="outbound" type="password"><button id="save">保存</button><div id="result" class="status"></div><h2>Cloud provider 配置参考</h2><pre>` + template.HTMLEscapeString(templateValues) + `</pre><script>
const passwordInput=document.getElementById('password');const inboundInput=document.getElementById('inbound');const outboundInput=document.getElementById('outbound');const result=document.getElementById('result');
document.getElementById('save').onclick=async()=>{const r=await fetch('/integration-demo/api/setup',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({password:passwordInput.value,inbound_secret:inboundInput.value,outbound_secret:outboundInput.value})});const d=await r.json();result.textContent=r.ok?'配置已保存，密钥不会再次显示。':(d.message||'保存失败');if(r.ok){inboundInput.value='';outboundInput.value=''}}
</script>`
	renderHTML(w, page("Demo 设置", body))
}

func (s *server) setupStatus(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"configured": s.configured()})
}
func (s *server) saveSetup(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Password       string `json:"password"`
		InboundSecret  string `json:"inbound_secret"`
		OutboundSecret string `json:"outbound_secret"`
	}
	if json.NewDecoder(io.LimitReader(r.Body, 8192)).Decode(&input) != nil {
		writeJSON(w, 400, map[string]string{"message": "请求格式错误"})
		return
	}
	if input.Password != s.adminPassword {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"message": "管理密码错误"})
		return
	}
	if input.InboundSecret == "" || input.OutboundSecret == "" {
		writeJSON(w, 400, map[string]string{"message": "两条 HMAC 密钥均为必填"})
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.Configuration = configuration{InboundSecret: input.InboundSecret, OutboundSecret: input.OutboundSecret}
	if err := s.saveLocked(); err != nil {
		writeJSON(w, 500, map[string]string{"message": "配置保存失败"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

func (s *server) createOrder(w http.ResponseWriter, r *http.Request) {
	if !s.configured() {
		writeJSON(w, 503, map[string]string{"message": "Demo 尚未配置 HMAC 密钥"})
		return
	}
	if err := r.ParseMultipartForm(12 << 20); err != nil {
		writeJSON(w, 400, map[string]string{"message": "表单或文件过大"})
		return
	}
	ticket := r.FormValue("terminal_ticket")
	if ticket == "" {
		writeJSON(w, 400, map[string]string{"message": "终端票据已失效，请重新扫码"})
		return
	}
	content, name, err := uploadedPDF(r.MultipartForm.File["file"])
	if err != nil {
		writeJSON(w, 400, map[string]string{"message": err.Error()})
		return
	}
	id := randomID()
	fileID := randomID()
	path := filepath.Join(s.dataDir, "files", fileID+".pdf")
	if err := os.WriteFile(path, content, 0600); err != nil {
		writeJSON(w, 500, map[string]string{"message": "文件保存失败"})
		return
	}
	expires := time.Now().Add(10 * time.Minute).Unix()
	digest := sha256.Sum256(content)
	requestBody := map[string]any{"external_order_id": id, "external_user_id": "DEMO001", "external_user_name": "张老师", "terminal_ticket": ticket,
		"file":          map[string]any{"url": fmt.Sprintf("http://integration-demo:8080/files/%s?expires=%d", fileID, expires), "expires_at": time.Unix(expires, 0).UTC().Format(time.RFC3339), "name": name, "size": len(content), "mime_type": "application/pdf", "sha256": hex.EncodeToString(digest[:])},
		"print_options": map[string]any{"copies": 1, "paper_size": "A4", "color_mode": "grayscale", "duplex_mode": "single"}, "metadata": map[string]string{"business_type": "demo", "business_id": id}}
	raw, _ := json.Marshal(requestBody)
	response, err := s.flyPrintRequest(http.MethodPost, "/api/v1/integrations/"+providerCode+"/print-requests", raw)
	if err != nil {
		_ = os.Remove(path)
		writeJSON(w, 502, map[string]string{"message": "FlyPrint 拒绝了打印请求：" + err.Error()})
		return
	}
	var responseMap map[string]any
	_ = json.Unmarshal(response, &responseMap)
	requestID, _ := responseMap["request_id"].(string)
	status, _ := responseMap["status"].(string)
	s.mu.Lock()
	current := s.state.Orders[id]
	if current.Status == "" {
		current.Status = status
	}
	current.ID = id
	current.RequestID = requestID
	current.FileName = name
	current.UpdatedAt = time.Now()
	s.state.Orders[id] = current
	_ = s.saveLocked()
	s.mu.Unlock()
	writeJSON(w, http.StatusAccepted, map[string]string{"order_id": id, "request_id": requestID, "status": status})
}

func uploadedPDF(files []*multipart.FileHeader) ([]byte, string, error) {
	if len(files) == 0 {
		return samplePDF(), "丽娃云聘测试材料.pdf", nil
	}
	file, err := files[0].Open()
	if err != nil {
		return nil, "", fmt.Errorf("无法读取 PDF")
	}
	defer file.Close()
	raw, err := io.ReadAll(io.LimitReader(file, (10<<20)+1))
	if err != nil || len(raw) > 10<<20 {
		return nil, "", fmt.Errorf("PDF 不得超过 10 MB")
	}
	if !strings.HasPrefix(string(raw), "%PDF-") {
		return nil, "", fmt.Errorf("仅支持有效 PDF 文件")
	}
	return raw, filepath.Base(files[0].Filename), nil
}

func (s *server) flyPrintRequest(method, path string, body []byte) ([]byte, error) {
	s.mu.RLock()
	secret := s.state.Configuration.InboundSecret
	s.mu.RUnlock()
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	nonce := randomID()
	signature := sign(secret, method, path, timestamp, nonce, body)
	req, _ := http.NewRequest(method, s.cloudBase+path, strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-FP-Client", providerCode)
	req.Header.Set("X-FP-Timestamp", timestamp)
	req.Header.Set("X-FP-Nonce", nonce)
	req.Header.Set("X-FP-Signature", signature)
	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return raw, nil
}

func (s *server) getOrder(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	item, ok := s.state.Orders[r.PathValue("id")]
	s.mu.RUnlock()
	if !ok {
		writeJSON(w, 404, map[string]string{"message": "订单不存在"})
		return
	}
	writeJSON(w, 200, item)
}
func (s *server) serveFile(w http.ResponseWriter, r *http.Request) {
	expires, _ := strconv.ParseInt(r.URL.Query().Get("expires"), 10, 64)
	if expires < time.Now().Unix() {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/pdf")
	http.ServeFile(w, r, filepath.Join(s.dataDir, "files", filepath.Base(r.PathValue("id"))+".pdf"))
}

func (s *server) callback(w http.ResponseWriter, r *http.Request) {
	raw, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		http.Error(w, "bad body", 400)
		return
	}
	s.mu.RLock()
	secret := s.state.Configuration.OutboundSecret
	s.mu.RUnlock()
	if r.Header.Get("X-FP-Client") != providerCode || !verify(secret, r.Header.Get("X-FP-Signature"), r.Method, "/api/print/callback", r.Header.Get("X-FP-Timestamp"), r.Header.Get("X-FP-Nonce"), raw) {
		http.Error(w, "unauthorized", 401)
		return
	}
	var event struct {
		EventID         string `json:"event_id"`
		ExternalOrderID string `json:"external_order_id"`
		Status          string `json:"status"`
		ErrorCode       string `json:"error_code"`
		ErrorMessage    string `json:"error_message"`
	}
	if json.Unmarshal(raw, &event) != nil || event.EventID == "" {
		http.Error(w, "bad json", 400)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state.Events[event.EventID] {
		w.WriteHeader(204)
		return
	}
	s.state.Events[event.EventID] = true
	item := s.state.Orders[event.ExternalOrderID]
	item.ID = event.ExternalOrderID
	item.Status = event.Status
	item.ErrorCode = event.ErrorCode
	item.ErrorMessage = event.ErrorMessage
	item.UpdatedAt = time.Now()
	s.state.Orders[event.ExternalOrderID] = item
	_ = s.saveLocked()
	w.WriteHeader(204)
}

func sign(secret, method, path, timestamp, nonce string, body []byte) string {
	sum := sha256.Sum256(body)
	canonical := strings.Join([]string{method, path, timestamp, nonce, hex.EncodeToString(sum[:])}, "\n")
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(canonical))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}
func verify(secret, signature, method, path, timestamp, nonce string, body []byte) bool {
	seconds, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil || time.Since(time.Unix(seconds, 0)).Abs() > 5*time.Minute || nonce == "" {
		return false
	}
	received, err := base64.StdEncoding.DecodeString(signature)
	if err != nil {
		return false
	}
	expected, _ := base64.StdEncoding.DecodeString(sign(secret, method, path, timestamp, nonce, body))
	return hmac.Equal(received, expected)
}
func randomID() string { b := make([]byte, 16); _, _ = rand.Read(b); return hex.EncodeToString(b) }
func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
func schemeHost(r *http.Request) string {
	scheme := "http"
	if value := r.Header.Get("X-Forwarded-Proto"); value != "" {
		scheme = value
	}
	host := r.Header.Get("X-Forwarded-Host")
	if host == "" {
		host = r.Host
	}
	return scheme + "://" + host
}
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func renderHTML(w http.ResponseWriter, value string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = io.WriteString(w, value)
}
func renderError(w http.ResponseWriter, title, message string, status int) {
	w.WriteHeader(status)
	renderHTML(w, page(title, "<h1>"+template.HTMLEscapeString(title)+"</h1><p>"+template.HTMLEscapeString(message)+"</p>"))
}
func page(title, body string) string {
	return `<!doctype html><html lang="zh-CN"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>` + template.HTMLEscapeString(title) + `</title><style>*{box-sizing:border-box}body{margin:0;background:#f3f6fb;color:#172033;font-family:system-ui,-apple-system,"Microsoft YaHei",sans-serif}main{max-width:620px;margin:40px auto;padding:28px;background:#fff;border-radius:18px;box-shadow:0 12px 36px #21395f18}h1{margin-top:0}h2{margin-top:28px;font-size:18px}.muted,p{color:#657087;line-height:1.7}label{display:block;margin:16px 0 6px;font-weight:600}input{width:100%;padding:12px;border:1px solid #ccd5e3;border-radius:10px}button{width:100%;margin-top:22px;padding:14px;border:0;border-radius:11px;background:#1769e0;color:#fff;font-size:16px;cursor:pointer}button:disabled{opacity:.6}.status{margin-top:18px;padding:14px;border-radius:10px;background:#eef5ff;color:#1558b0;min-height:48px}pre{white-space:pre-wrap;word-break:break-all;background:#f6f8fb;padding:12px;border-radius:10px}</style></head><body><main>` + body + `</main></body></html>`
}
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}
func (s *server) cleanupFiles() {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for range ticker.C {
		entries, _ := os.ReadDir(filepath.Join(s.dataDir, "files"))
		for _, entry := range entries {
			info, err := entry.Info()
			if err == nil && time.Since(info.ModTime()) > 30*time.Minute {
				_ = os.Remove(filepath.Join(s.dataDir, "files", entry.Name()))
			}
		}
	}
}

func samplePDF() []byte {
	objects := []string{
		"<</Type/Catalog/Pages 2 0 R>>",
		"<</Type/Pages/Kids[3 0 R]/Count 1>>",
		"<</Type/Page/Parent 2 0 R/MediaBox[0 0 595 842]/Resources<</Font<</F1 4 0 R>>>>/Contents 5 0 R>>",
		"<</Type/Font/Subtype/Type1/BaseFont/Helvetica>>",
	}
	stream := "BT /F1 24 Tf 72 760 Td (FlyPrint Integration Demo) Tj 0 -40 Td /F1 14 Tf (Test document) Tj ET\n"
	objects = append(objects, fmt.Sprintf("<</Length %d>>stream\n%sendstream", len(stream), stream))

	var document bytes.Buffer
	document.WriteString("%PDF-1.4\n")
	offsets := make([]int, len(objects)+1)
	for index, object := range objects {
		offsets[index+1] = document.Len()
		fmt.Fprintf(&document, "%d 0 obj\n%s\nendobj\n", index+1, object)
	}
	xrefOffset := document.Len()
	fmt.Fprintf(&document, "xref\n0 %d\n0000000000 65535 f \n", len(objects)+1)
	for index := 1; index < len(offsets); index++ {
		fmt.Fprintf(&document, "%010d 00000 n \n", offsets[index])
	}
	fmt.Fprintf(&document, "trailer\n<</Size %d/Root 1 0 R>>\nstartxref\n%d\n%%%%EOF\n", len(objects)+1, xrefOffset)
	return document.Bytes()
}
