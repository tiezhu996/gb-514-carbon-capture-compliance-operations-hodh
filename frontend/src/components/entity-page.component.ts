import { AsyncPipe, CommonModule } from '@angular/common';
import { ChangeDetectorRef, Component, Input, OnInit } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { MatButtonModule } from '@angular/material/button';
import { MatInputModule } from '@angular/material/input';
import { authState } from '../hooks/use-auth';
import type { EntityStore } from '../stores/factory';
import type { DomainRecord, EntityConfig } from '../types/domain';
import { formatDate, nextStatus } from '../utils/format';
import { ConfirmDialogComponent } from './common/confirm-dialog.component';
import { ComplianceBadgeComponent } from './common/compliance-badge.component';
import { EvidenceListComponent } from './common/evidence-list.component';
import { MetricCardComponent } from './common/metric-card.component';
import { RuleDiffComponent } from './common/rule-diff.component';
import { StatusBadgeComponent } from './common/status-badge.component';

@Component({
  selector: 'app-entity-page',
  standalone: true,
  imports: [CommonModule, AsyncPipe, FormsModule, MatButtonModule, MatInputModule, StatusBadgeComponent, MetricCardComponent, ConfirmDialogComponent, EvidenceListComponent, ComplianceBadgeComponent, RuleDiffComponent],
  template: `<main class="workspace" *ngIf="store.state$ | async as state">
    <header class="page-header">
      <div><p class="eyebrow">业务工作台</p><h1>{{ config.label }}</h1><p>统一管理{{ config.label }}的状态、风险、证据与责任人。</p></div>
      <button *ngIf="canWrite()" mat-flat-button color="primary" (click)="openCreate()">新增{{ config.label }}</button>
    </header>
    <section class="metrics">
      <app-metric-card label="记录总数" [value]="state.meta.total" detail="当前筛选范围"/>
      <app-metric-card label="高风险" [value]="highRisk(state.items)" detail="需要优先复核"/>
      <app-metric-card label="状态种类" [value]="statusCount(state.items)" detail="状态机覆盖"/>
    </section>
    <app-compliance-badge *ngIf="config.path === 'units' || config.path === 'decisions'" [records]="state.items"/>
    <app-rule-diff *ngIf="config.path === 'rules' || config.path === 'samples'" [records]="state.items"/>
    <app-evidence-list [records]="state.items"/>
    <section class="toolbar">
      <input matInput aria-label="搜索" [(ngModel)]="search" [placeholder]="'搜索' + config.label + '编码或名称'"/>
      <button mat-flat-button color="primary" (click)="query()">查询</button>
      <button mat-button (click)="reset()">重置</button>
    </section>
    <div *ngIf="state.error" class="alert">{{ state.error }}</div>
    <section class="table-shell"><table><thead><tr><th>编码</th><th>名称</th><th>状态</th><th>风险</th><th>责任人</th><th>指标</th><th>更新时间</th><th>操作</th></tr></thead>
      <tbody><tr *ngFor="let item of state.items"><td><strong>{{ item.code }}</strong></td><td>{{ item.name }}<small>{{ item.facility }}</small></td><td><app-status-badge [status]="item.status"/></td><td>{{ item.riskLevel }}</td><td>{{ item.owner }}</td><td>{{ item.metricValue }} {{ item.metricUnit }}</td><td>{{ formatDate(item.updatedAt) }}</td><td>
        <button *ngIf="canTransition(item)" class="table-action" (click)="openTransition(item, next(item)!)">推进至 {{ next(item) }}</button>
        <span *ngIf="!canTransition(item)" class="muted">{{ actionHint(item) }}</span>
      </td></tr><tr *ngIf="!state.items.length && !state.loading"><td colspan="8" class="empty">暂无记录</td></tr></tbody></table>
      <div *ngIf="state.loading" class="loading">正在同步业务数据…</div>
    </section>
    <app-confirm-dialog [open]="showCreate" [title]="'新增' + config.label" (cancel)="closeCreate()" (confirm)="createDemo()"><p>将创建一条包含完整责任人、风险和证据信息的演示记录。</p></app-confirm-dialog>
    <app-confirm-dialog [open]="!!pending" title="确认状态迁移" (cancel)="closeTransition()" (confirm)="confirmTransition()"><p>状态迁移会追加不可变版本并记录证据、操作者与请求 ID。</p><strong>{{ pending?.item?.status }} → {{ pending?.status }}</strong></app-confirm-dialog>
  </main>`,
})
export class EntityPageComponent implements OnInit {
  @Input({ required: true }) config!: EntityConfig;
  @Input({ required: true }) store!: EntityStore;
  search = '';
  showCreate = false;
  pending: { item: DomainRecord; status: string } | null = null;
  readonly formatDate = formatDate;

  constructor(private readonly changeDetector: ChangeDetectorRef) {}
  async ngOnInit() { await this.load(); }
  next(item: DomainRecord) { return nextStatus(item.status, this.config.statuses); }
  canWrite() { return authState.hasMinimumRole('operator'); }
  canReview() { return authState.hasMinimumRole('reviewer'); }
  canTransition(item: DomainRecord): boolean {
    const target = this.next(item);
    if (!target || !this.canWrite()) return false;
    if (this.config.path === 'decisions' && ['accepted', 'escalated'].includes(target)) return this.canReview();
    return true;
  }
  actionHint(item: DomainRecord): string {
    if (!this.canWrite()) return '只读权限';
    const target = this.next(item);
    if (this.config.path === 'decisions' && target && ['accepted', 'escalated'].includes(target) && !this.canReview()) return '等待复核员决定';
    return '流程结束';
  }
  highRisk(items: DomainRecord[]) { return items.filter((item) => ['high', 'critical'].includes(item.riskLevel)).length; }
  statusCount(items: DomainRecord[]) { return new Set(items.map((item) => item.status)).size; }
  async query() { await this.load(this.search); }
  async reset() { this.search = ''; await this.load(); }
  openCreate() { if (this.canWrite()) this.showCreate = true; this.changeDetector.detectChanges(); }
  closeCreate() { this.showCreate = false; this.changeDetector.detectChanges(); }
  openTransition(item: DomainRecord, status: string) { if (this.canTransition(item)) this.pending = { item, status }; this.changeDetector.detectChanges(); }
  closeTransition() { this.pending = null; this.changeDetector.detectChanges(); }
  async createDemo() {
    if (!this.canWrite()) return;
    const now = Date.now();
    try {
      await this.store.createRecord(this.config.path, {
        code: `${this.config.key.toUpperCase()}-${String(now).slice(-6)}`, name: `新增${this.config.label}`,
        description: '通过前端工作台创建的业务记录', facility: '默认作业区', owner: '现场操作员',
        category: '常规', riskLevel: 'medium', metricValue: 25, metricUnit: 'unit',
        effectiveAt: new Date().toISOString(), evidence: '已完成创建前检查', relatedCode: '',
      });
	  this.search = '';
      this.showCreate = false;
    } catch { /* Store exposes the request error in its observable state. */ }
    finally { this.changeDetector.detectChanges(); }
  }
  async confirmTransition() {
    if (!this.pending || !this.canTransition(this.pending.item)) return;
    try {
      await this.store.transition(this.config.path, this.pending.item, this.pending.status);
	  this.search = '';
      this.pending = null;
    } catch { /* Store exposes the request error in its observable state. */ }
    finally { this.changeDetector.detectChanges(); }
  }
  private async load(search = '') { await this.store.load(this.config.path, search); this.changeDetector.detectChanges(); }
}
