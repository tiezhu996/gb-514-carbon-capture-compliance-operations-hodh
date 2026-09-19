import { CommonModule } from '@angular/common';
import { Component, Input } from '@angular/core';
import type { DomainRecord } from '../../types/domain';
import { StatusBadgeComponent } from './status-badge.component';

@Component({
  selector: 'app-compliance-badge',
  standalone: true,
  imports: [CommonModule, StatusBadgeComponent],
  template: `<section class="compliance-board" aria-label="合规态势">
    <header><strong>合规态势</strong><span>{{ attentionCount }} 项需要优先复核</span></header>
    <div class="compliance-grid">
      <article *ngFor="let item of records.slice(0, 4)">
        <div><strong>{{ item.code }}</strong><span>{{ item.name }}</span></div>
        <em [class]="'risk risk--' + item.riskLevel">{{ item.riskLevel }}</em>
        <app-status-badge [status]="item.status"/>
      </article>
    </div>
  </section>`,
})
export class ComplianceBadgeComponent {
  @Input() records: DomainRecord[] = [];
  get attentionCount(): number {
    return this.records.filter((item) => ['high', 'critical'].includes(item.riskLevel) || ['limited', 'escalated'].includes(item.status)).length;
  }
}
