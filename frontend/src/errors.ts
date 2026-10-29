import messages from '../../internal/apperrors/messages.json';

export function codedError(code: keyof typeof messages): Error {
  return new Error(messages[code], { cause: { code } });
}

// Accept only a closed code from our service envelope. Never render a bridge,
// transport, parser or third-party message, even if it looks like safe Chinese.
export function errorMessage(reason: unknown): string {
  if (reason && typeof reason === 'object') {
    const cause = 'cause' in reason ? reason.cause : reason;
    if (cause && typeof cause === 'object' && 'code' in cause &&
        typeof cause.code === 'string' && Object.hasOwn(messages, cause.code)) {
      return messages[cause.code as keyof typeof messages];
    }
  }
  return messages.Unexpected;
}

export function retryTime(reason: unknown): number {
  if (!reason || typeof reason!=='object' || !('cause' in reason)) return 0;
  const cause=reason.cause;
  if(!cause || typeof cause!=='object' || !('code' in cause) || !('retryAt' in cause))return 0;
  if(typeof cause.code!=='string' || !['NotLoggedIn','RateLimited','ServiceBusy','NetworkUnavailable','RequestTimeout'].includes(cause.code) || typeof cause.retryAt!=='string')return 0;
  return Date.parse(cause.retryAt)||0;
}
