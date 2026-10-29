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
