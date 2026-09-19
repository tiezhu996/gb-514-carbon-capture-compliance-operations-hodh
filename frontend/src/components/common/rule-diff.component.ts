import { CommonModule } from '@angular/common';
import { Component, Input } from '@angular/core';
import type { DomainRecord } from '../../types/domain';

@Component({
  selector: 'app-rule-diff',
  standalone: true,
  imports: [CommonModule],
  template: `<section class="rule-diff" aria-label="许可阈值对比">
    <header><strong>许可阈值对比</strong><span>以首条记录作为当前筛选基线</span></header>
    <div class="rule-diff-grid" *ngIf="records.length; else empty">
      <article *ngFor="let item of records.slice(0, 4)">
        <strong>{{ item.code }}</strong><span>{{ item.metricValue }} {{ item.metricUnit }}</span>
        <em [class.over-limit]="delta(item) > 0">{{ signedDelta(item) }}</em>
      </article>
    </div>
    <ng-template #empty><div class="empty">暂无可比较数据</div></ng-template>
  </section>`,
})
export class RuleDiffComponent {
  @Input() records: DomainRecord[] = [];
  delta(item: DomainRecord): number { return item.metricValue - (this.records[0]?.metricValue || 0); }
  signedDelta(item: DomainRecord): string {
    const value = this.delta(item);
    return `${value > 0 ? '+' : ''}${value.toFixed(1)}`;
  }
}
