
export interface DomainRecord {
  id: number;
  code: string;
  name: string;
  status: string;
  version: number;
  description: string;
  facility: string;
  owner: string;
  category: string;
  riskLevel: 'low' | 'medium' | 'high' | 'critical';
  metricValue: number;
  metricUnit: string;
  effectiveAt: string;
  evidence: string;
  relatedCode: string;
  permitRuleCode?: string;
  permitRuleVersion?: number;
  createdAt: string;
  updatedAt: string;
  revisions?: DecisionRevision[];
}

export interface DecisionRevision {
  id: number;
  complianceDecisionId: number;
  version: number;
  state: string;
  evidence: string;
  reason: string;
  permitRuleCode?: string;
  permitRuleVersion?: number;
  actor: string;
  requestId: string;
  createdAt: string;
}

export interface RetirementBlocker {
  code: string;
  decisionCode?: string;
  reason: string;
}

export interface RetirementCheck {
  permitRuleCode: string;
  relatedCode: string;
  allowed: boolean;
  deviceCode?: string;
  sampleCode?: string;
  blockers: RetirementBlocker[];
}

export interface PageMeta { page: number; pageSize: number; total: number }
export interface ApiEnvelope<T> { data: T; error?: string; message?: string; meta?: PageMeta }
export interface UserSession { token: string; username: string; displayName: string; role: string; expiresIn: number }
export interface AuditLog {
  id: number; requestId: string; actor: string; action: string; entityType: string;
  entityId: number; beforeState: string; afterState: string; detail: string; createdAt: string;
}
export interface EntityConfig { key: string; path: string; label: string; statuses: readonly string[] }
