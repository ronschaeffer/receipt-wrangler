import { NgModule } from "@angular/core";
import { RouterModule, Routes } from "@angular/router";

import { ReportTableComponent } from "./report-table/report-table.component";
import { ReportDetailComponent } from "./report-detail/report-detail.component";

const routes: Routes = [
  {
    path: "group/:groupId",
    component: ReportTableComponent,
  },
  {
    path: "group/:groupId/report/:reportId",
    component: ReportDetailComponent,
  },
];

@NgModule({
  imports: [RouterModule.forChild(routes)],
  exports: [RouterModule],
})
export class ReportsRoutingModule {}
