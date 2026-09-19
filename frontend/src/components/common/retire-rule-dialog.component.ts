import { CommonModule } from '@angular/common';
import { Component, EventEmitter, Input, Output } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { MatButtonModule } from '@angular/material/button';
import { MatInputModule } from '@angular/material/input';
import type { DomainRecord, RetirementBlocker, RetirementCheck } from '../../types/domain';

@Component({
  selector: 'app-retire-rule-dialog',
  standalone: true,
  imports: [CommonModule, FormsModule, MatButtonModule, MatInputModule],
  template: `<div *ngIf="open" class="modal-backdrop"><section class="modal retire-modal" role="dialog" aria-label="许可规则作废核验">
    <h2>作废许可规则 {{ rule?.code }}</h2>
    <p class="retire-hint">作废前将按装置重新核验在途合规决定、最新已核验样本与装置归属；仅管理员可执行，旧规则保留为历史记录。</p>
    <div *ngIf="loading" class="retire-loading">正在按装置启动核验…</div>
    <ng-container *ngIf="!loading && check">
      <div *ngIf="check.allowed; else blocked" class="retire-ok">
        <strong>核验通过，允许作废</strong>
        <ul>
          <li>关联装置：{{ check.deviceCode || '未读取' }}（{{ check.relatedCode }}）</li>
          <li>最新已核验样本：{{ check.sampleCode || '未读取' }}</li>
          <li>无草稿或复核中的关联合规决定</li>
        </ul>
        <label class="retire-reason">作废原因
          <textarea matInput [(ngModel)]="reason" rows="2" maxlength="500" placeholder="请填写作废原因（至少 3 个字符）"></textarea>
        </label>
      </div>
      <ng-template #blocked>
        <div class="retire-blocked">
          <strong>核验未通过，作废已拒绝（{{ check.blockers.length }} 项阻断）</strong>
          <ul class="blocker-list">
            <li *ngFor="let blocker of check.blockers">
              <span class="blocker-code">{{ label(blocker) }}</span>
              <span *ngIf="blocker.decisionCode" class="blocker-decision">决定编号 {{ blocker.decisionCode }}</span>
              <p>{{ blocker.reason }}</p>
            </li>
          </ul>
        </div>
      </ng-template>
    </ng-container>
    <div *ngIf="submitting" class="retire-loading">正在提交作废事务…</div>
    <footer>
      <button mat-button (click)="cancel.emit()" [disabled]="submitting">关闭</button>
      <button *ngIf="check?.allowed" mat-flat-button color="primary" (click)="confirmRetire()"
              [disabled]="submitting || reason.trim().length < 3">确认作废</button>
    </footer>
  </section></div>`,
})
export class RetireRuleDialogComponent {
  @Input() open = false;
  @Input() rule: DomainRecord | null = null;
  @Input() check: RetirementCheck | null = null;
  @Input() loading = false;
  @Input() submitting = false;
  @Output() confirm = new EventEmitter<string>();
  @Output() cancel = new EventEmitter<void>();
  reason = '';

  confirmRetire(): void {
    if (this.reason.trim().length >= 3) this.confirm.emit(this.reason.trim());
  }

  label(blocker: RetirementBlocker): string {
    switch (blocker.code) {
      case 'in_flight_decision': return '在途决定';
      case 'device_missing': return '装置缺失';
      case 'sample_missing': return '样本缺失';
      case 'device_mismatch': return '装置不一致';
      case 'device_read_failed': return '装置读取失败';
      case 'sample_read_failed': return '样本读取失败';
      case 'rule_state': return '规则状态';
      default: return '阻断';
    }
  }
}
