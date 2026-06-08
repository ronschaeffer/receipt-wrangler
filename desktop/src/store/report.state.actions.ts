import { Report } from '../open-api/model/report';

export class SetReports {
  static readonly type = '[Report] Set Reports';
  constructor(public reports: Report[]) {}
}

export class AddReport {
  static readonly type = '[Report] Add Report';
  constructor(public report: Report) {}
}

export class UpdateReport {
  static readonly type = '[Report] Update Report';
  constructor(public report: Report) {}
}

export class RemoveReport {
  static readonly type = '[Report] Remove Report';
  constructor(public reportId: number) {}
}

export class SetSelectedReportId {
  static readonly type = '[Report] Set Selected Report Id';
  constructor(public reportId?: number) {}
}
