package integration

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"strconv"
	"strings"
	"time"

	"fly-print-cloud/api/internal/database"
	"fly-print-cloud/api/internal/security"
)

type CallbackWorker struct { events *database.IntegrationCallbackRepository; providers *database.IntegrationProviderRepository; cipher *security.ClientSecretCipher }
func NewCallbackWorker(events *database.IntegrationCallbackRepository, providers *database.IntegrationProviderRepository, cipher *security.ClientSecretCipher) *CallbackWorker{return &CallbackWorker{events,providers,cipher}}
func (w *CallbackWorker) Run(ctx context.Context) { ticker:=time.NewTicker(time.Second);defer ticker.Stop();for{_ = w.ProcessOne(ctx);select{case<-ctx.Done():return;case<-ticker.C:}} }
func (w *CallbackWorker) ProcessOne(ctx context.Context) error {
	event,err:=w.events.ClaimDue(time.Now());if err!=nil||event==nil{return err}
	provider,err:=w.providers.Get(event.ProviderCode,true);if err!=nil||provider==nil||!provider.Enabled{return w.events.Retry(event.ID,event.AttemptCount)}
	secret,err:=w.cipher.Decrypt(provider.OutboundSecretEncrypted);if err!=nil{return w.events.Retry(event.ID,event.AttemptCount)}
	nonceBytes:=make([]byte,24);if _,err=rand.Read(nonceBytes);err!=nil{return w.events.Retry(event.ID,event.AttemptCount)}
	nonce:=base64.RawURLEncoding.EncodeToString(nonceBytes);timestamp:=strconv.FormatInt(time.Now().Unix(),10)
	req,err:=http.NewRequestWithContext(ctx,http.MethodPost,event.TargetURL,strings.NewReader(event.Payload));if err!=nil{return w.events.Retry(event.ID,event.AttemptCount)}
	req.Header.Set("Content-Type","application/json");req.Header.Set("X-FP-Client",provider.Code);req.Header.Set("X-FP-Timestamp",timestamp);req.Header.Set("X-FP-Nonce",nonce);req.Header.Set("X-FP-Signature",Sign(secret,http.MethodPost,"/api/print/callback",timestamp,nonce,[]byte(event.Payload)))
	response,err:=(&http.Client{Timeout:10*time.Second}).Do(req);if err==nil&&response!=nil{response.Body.Close();if response.StatusCode>=200&&response.StatusCode<300{return w.events.Complete(event.ID)}}
	return w.events.Retry(event.ID,event.AttemptCount)
}
