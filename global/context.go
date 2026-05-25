package global

import "context"

// 💡 设置不可到处的内部变量
type traceKeyType struct{}

// 定义全局唯一的常量 Key 实例
var traceKey = traceKeyType{}

// WithTraceId 💡 强类型缝口袋：往 context 里安全注入 TraceID
func WithTraceId(ctx context.Context, traceId string) context.Context {
	return context.WithValue(ctx, traceKey, traceId)
}

// GetTraceId 💡 强类型掏口袋：从 context 里安全取出 TraceID
func GetTraceId(ctx context.Context) string {
	if val, ok := ctx.Value(traceKey).(string); ok {
		return val
	}
	return ""
}

// DetachContext 💡 异步解耦圣杯：开后台协程异步干活时，只复制 TraceID，不复制生命周期
// 这样能确保主线程 HTTP 请求结束后，后台并发任务的链路追踪依然不会崩断
func DetachContext(ctx context.Context) context.Context {
	bgCtx := context.Background()
	traceId := GetTraceId(ctx)
	return WithTraceId(bgCtx, traceId)
}
