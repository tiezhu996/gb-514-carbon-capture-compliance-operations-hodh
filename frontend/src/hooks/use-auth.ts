import { signal } from '@angular/core';
import { login } from '../api/auth';
import { clearSession, getStoredSession, saveSession } from '../api/client';
import type { UserSession } from '../types/domain';

const roleRank: Record<string, number> = { viewer: 1, operator: 2, reviewer: 3, admin: 4 };
const session = signal<UserSession | null>(getStoredSession());
const loading = signal(!session());
let authentication: Promise<UserSession> | null = null;

async function authenticate(force = false): Promise<UserSession> {
  if (!force && session()) return session()!;
  if (!authentication) {
    loading.set(true);
    authentication = login().then((next) => {
      saveSession(next);
      session.set(next);
      return next;
    }).finally(() => {
      loading.set(false);
      authentication = null;
    });
  }
  return authentication;
}

function hasMinimumRole(minimum: string): boolean {
  return (roleRank[session()?.role || ''] || 0) >= (roleRank[minimum] || Number.MAX_SAFE_INTEGER);
}

function logout(): void {
  clearSession();
  session.set(null);
  void authenticate(true);
}

export const authState = { session, loading, authenticate, logout, hasMinimumRole };
export function createAuthState() { return authState; }
