import { Injectable } from '@angular/core';
import {
  Action,
  createSelector,
  Selector,
  State,
  StateContext,
} from '@ngxs/store';
import { Report } from '../open-api/model/report';
import {
  AddReport,
  RemoveReport,
  SetReports,
  SetSelectedReportId,
  UpdateReport,
} from './report.state.actions';

export interface ReportStateInterface {
  reports: Report[];
  selectedReportId: number | null;
}

@State<ReportStateInterface>({
  name: 'reports',
  defaults: {
    reports: [],
    selectedReportId: null,
  },
})
@Injectable()
export class ReportState {
  @Selector()
  static reports(state: ReportStateInterface): Report[] {
    return state.reports;
  }

  @Selector()
  static selectedReportId(state: ReportStateInterface): number | null {
    return state.selectedReportId;
  }

  @Selector()
  static selectedReport(state: ReportStateInterface): Report | undefined {
    return state.reports.find((r) => r.id === state.selectedReportId);
  }

  static getReportById(reportId: number) {
    return createSelector([ReportState], (state: ReportStateInterface) => {
      return state.reports.find((r) => r.id === reportId);
    });
  }

  @Action(SetReports)
  setReports(
    { patchState }: StateContext<ReportStateInterface>,
    payload: SetReports
  ) {
    patchState({
      reports: payload.reports,
    });
  }

  @Action(AddReport)
  addReport(
    { getState, patchState }: StateContext<ReportStateInterface>,
    payload: AddReport
  ) {
    const reports = Array.from(getState().reports);
    reports.push(payload.report);

    patchState({
      reports: reports,
    });
  }

  @Action(UpdateReport)
  updateReport(
    { getState, patchState }: StateContext<ReportStateInterface>,
    payload: UpdateReport
  ) {
    const reportIndex = getState().reports.findIndex(
      (r) => r.id === payload?.report?.id
    );
    if (reportIndex > -1) {
      const newReports = Array.from(getState().reports);
      newReports[reportIndex] = payload.report;

      patchState({
        reports: newReports,
      });
    }
  }

  @Action(RemoveReport)
  removeReport(
    { getState, patchState }: StateContext<ReportStateInterface>,
    payload: RemoveReport
  ) {
    const state = getState();
    const newInterface = {} as ReportStateInterface;
    newInterface.reports = Array.from(state.reports).filter(
      (r) => r.id !== payload.reportId
    );
    if (state.selectedReportId === payload.reportId) {
      newInterface.selectedReportId = null;
    }
    patchState(newInterface);
  }

  @Action(SetSelectedReportId)
  setSelectedReportId(
    { patchState }: StateContext<ReportStateInterface>,
    payload: SetSelectedReportId
  ) {
    patchState({
      selectedReportId: payload.reportId ?? null,
    });
  }
}
