import { CommonModule } from "@angular/common";
import { NgModule } from "@angular/core";
import { ReactiveFormsModule } from "@angular/forms";
import { SharedUiModule } from "src/shared-ui/shared-ui.module";
import { TableModule } from "src/table/table.module";
import { ButtonModule } from "../button";
import { DirectivesModule } from "../directives";
import { InputModule } from "../input";
import { PipesModule } from "../pipes";
import { ReportsRoutingModule } from "./reports-routing.module";
import { ReportFormComponent } from "./report-form/report-form.component";
import { ReportTableComponent } from "./report-table/report-table.component";
import { ReportDetailComponent } from "./report-detail/report-detail.component";

@NgModule({
  declarations: [
    ReportTableComponent,
    ReportDetailComponent,
    ReportFormComponent,
  ],
  imports: [
    ReportsRoutingModule,
    CommonModule,
    DirectivesModule,
    InputModule,
    PipesModule,
    ReactiveFormsModule,
    SharedUiModule,
    ButtonModule,
    TableModule,
  ],
})
export class ReportsModule {}
