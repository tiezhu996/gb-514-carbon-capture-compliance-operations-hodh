import { CommonModule } from '@angular/common';
import { Component, Input } from '@angular/core';
import type { DecisionRevision, DomainRecord } from '../../types/domain';

@Component({
  selector: 'app-evidence-list',
  standalone: true,
  imports: [CommonModule],
  template: `<section class="evidence-panel" aria-label="证据摘要">
    <header><strong>证据摘要</strong><span>展示业务证据及最近一次不可变版本上下文</span></header>
    <div *ngIf="records.length; else empty" class="evidence-strip">
      <article *ngFor="let item of records.slice(0, 4)">
        <strong>{{ item.code }} · {{ item.name }}</strong>
        <p>{{ item.evidence || '尚未附加证据说明' }}</p>
        <small *ngIf="latest(item) as revision">v{{ revision.version }} · {{ revision.actor }} · {{ revision.requestId }}</small>
        <small *ngIf="!latest(item)">当前版本 v{{ item.version }}</small>
      </article>
    </div>
    <ng-template #empty><div class="empty">暂无业务证据</div></ng-template>
  </section>`,
})
export class EvidenceListComponent {
  @Input() records: DomainRecord[] = [];
  latest(item: DomainRecord): DecisionRevision | null {
    return item.revisions?.[item.revisions.length - 1] || null;
  }
}
