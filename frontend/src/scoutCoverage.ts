// Static text for ScoutReport coverage fields. Only fixed codes are mapped; an
// unknown reason falls back to one generic sentence and no raw message from the
// backend is ever rendered (U1).
const reasons: Record<string, string> = {
  RequestTimeout: "战绩服务响应超时",
  NetworkUnavailable: "网络连接不可用",
  ServiceBusy: "战绩服务暂时繁忙",
  RateLimited: "战绩服务请求过多",
  NotLoggedIn: "客户端登录状态未就绪",
  DataUnavailable: "战绩数据暂不可用",
  InvalidResponse: "战绩数据格式不符",
  range_limited: "当前记录范围受限",
  incomplete: "统计尚未读全",
};

export function scoutReason(reason?: string | null): string {
  if (!reason) return "";
  return reasons[reason] ?? "战绩数据暂不可用";
}

// Whole seconds until the announced retry, or null when none is announced.
export function scoutRetrySeconds(retryAt?: string | null, now: number = Date.now()): number | null {
  if (!retryAt) return null;
  const at = Date.parse(retryAt);
  if (Number.isNaN(at)) return null;
  const seconds = Math.ceil((at - now) / 1000);
  return seconds > 0 ? seconds : null;
}

export function scoutCoverageText(
  report: { reason?: string | null; retryAt?: string | null } | null | undefined,
  now: number = Date.now(),
): string {
  const reason = scoutReason(report?.reason);
  const seconds = scoutRetrySeconds(report?.retryAt, now);
  if (reason && seconds) return `${reason}，约 ${seconds} 秒后自动重试`;
  if (reason) return reason;
  return "统计尚未读全，可在主程序刷新数据";
}
