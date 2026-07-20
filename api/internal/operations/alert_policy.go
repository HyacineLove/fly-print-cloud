package operations

import (
  "time"

  "fly-print-cloud/api/internal/database"
)

type AlertPolicy struct {
  database.AlertSpec
  ActivationDelay time.Duration
  ConnectionScope bool
}

var alertPolicies = map[string]AlertPolicy{
  "printer_status_sync_interrupted": {AlertSpec: database.AlertSpec{Category: "sync", Title: "打印状态同步中断"}},
  "disk_usage_critical":             {AlertSpec: database.AlertSpec{Category: "resource", Title: "节点磁盘空间严重不足"}},
  "memory_usage_critical":           {AlertSpec: database.AlertSpec{Category: "resource", Title: "节点内存压力严重"}},
  "printer_out_of_paper":            {AlertSpec: database.AlertSpec{Category: "device", Title: "打印机缺纸"}},
  "printer_jammed":                  {AlertSpec: database.AlertSpec{Category: "device", Title: "打印机卡纸"}},
  "printer_out_of_toner":            {AlertSpec: database.AlertSpec{Category: "supply", Title: "打印机耗材已用尽"}},
  "printer_cover_open":              {AlertSpec: database.AlertSpec{Category: "device", Title: "打印机机盖打开"}},
  "ipp_unreachable":                 {AlertSpec: database.AlertSpec{Category: "connection", Title: "打印链路无法连接"}, ActivationDelay: 60 * time.Second, ConnectionScope: true},
  "printer_not_accepting_jobs":      {AlertSpec: database.AlertSpec{Category: "device", Title: "打印机拒绝接收任务"}, ActivationDelay: 60 * time.Second},
  "printer_stopped":                 {AlertSpec: database.AlertSpec{Category: "device", Title: "打印机已停止"}},
  "printer_user_intervention":       {AlertSpec: database.AlertSpec{Category: "device", Title: "打印机需要人工处理"}},
  "printer_other_fault":             {AlertSpec: database.AlertSpec{Category: "device", Title: "打印机发生故障"}},
  "printer_unconfirmed_lock":        {AlertSpec: database.AlertSpec{Category: "job", Title: "打印机存在结果未确认任务"}},
}

func alertPolicy(reason string) (AlertPolicy, bool) {
  policy, ok := alertPolicies[reason]
  return policy, ok
}

func policyReady(policy AlertPolicy, observedSince time.Time, now time.Time) bool {
  return policy.ActivationDelay <= 0 || !observedSince.IsZero() && now.Sub(observedSince) >= policy.ActivationDelay
}

func connectionScopedPrinterReasons() []string {
  return []string{"ipp_unreachable"}
}
