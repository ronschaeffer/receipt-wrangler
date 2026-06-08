import {
  AfterViewInit,
  Component,
  OnInit,
  signal,
  TemplateRef,
  viewChild,
} from "@angular/core";
import { MatDialog } from "@angular/material/dialog";
import { MatTableDataSource } from "@angular/material/table";
import { ActivatedRoute, Router } from "@angular/router";
import { Store } from "@ngxs/store";
import { of, switchMap, take, tap } from "rxjs";
import { DEFAULT_DIALOG_CONFIG } from "src/constants";
import { ConfirmationDialogComponent } from "src/shared-ui/confirmation-dialog/confirmation-dialog.component";
import { TableColumn } from "src/table/table-column.interface";
import { TableComponent } from "src/table/table/table.component";
import { Report, ReportService } from "../../open-api";
import { SnackbarService } from "../../services";
import {
  RemoveReport,
  SetReports,
} from "../../store/report.state.actions";
import { ReportState } from "../../store/report.state";
import { ReportFormComponent } from "../report-form/report-form.component";

@Component({
  selector: "app-report-table",
  templateUrl: "./report-table.component.html",
  styleUrls: ["./report-table.component.scss"],
  standalone: false,
})
export class ReportTableComponent implements OnInit, AfterViewInit {
  public readonly nameCell = viewChild.required<TemplateRef<any>>("nameCell");

  public readonly statusCell =
    viewChild.required<TemplateRef<any>>("statusCell");

  public readonly submittedDateCell =
    viewChild.required<TemplateRef<any>>("submittedDateCell");

  public readonly paidDateCell =
    viewChild.required<TemplateRef<any>>("paidDateCell");

  public readonly actionsCell =
    viewChild.required<TemplateRef<any>>("actionsCell");

  public reports = this.store.selectSignal(ReportState.reports);

  public dataSource = signal(new MatTableDataSource<Report>([]));

  public displayedColumns: string[] = [];

  public columns: TableColumn[] = [];

  public headerText: string = "Expense Reports";

  public groupId!: string;

  constructor(
    private reportService: ReportService,
    private matDialog: MatDialog,
    private snackbarService: SnackbarService,
    private store: Store,
    private activatedRoute: ActivatedRoute,
    private router: Router
  ) {}

  public ngOnInit(): void {
    this.groupId = this.activatedRoute.snapshot.params["groupId"];
    this.getReports();
  }

  public ngAfterViewInit(): void {
    this.setColumns();
  }

  private getReports(): void {
    this.reportService
      .getReportsForGroup(this.groupId)
      .pipe(
        take(1),
        tap((reports) => {
          this.store.dispatch(new SetReports(reports));
          this.dataSource.set(new MatTableDataSource<Report>(reports));
        })
      )
      .subscribe();
  }

  private setColumns(): void {
    this.columns = [
      {
        columnHeader: "Name",
        matColumnDef: "name",
        template: this.nameCell(),
        sortable: false,
      },
      {
        columnHeader: "Status",
        matColumnDef: "status",
        template: this.statusCell(),
        sortable: false,
      },
      {
        columnHeader: "Submitted",
        matColumnDef: "submittedDate",
        template: this.submittedDateCell(),
        sortable: false,
      },
      {
        columnHeader: "Paid",
        matColumnDef: "paidDate",
        template: this.paidDateCell(),
        sortable: false,
      },
      {
        columnHeader: "Actions",
        matColumnDef: "actions",
        template: this.actionsCell(),
        sortable: false,
      },
    ] as TableColumn[];

    this.displayedColumns = [
      "name",
      "status",
      "submittedDate",
      "paidDate",
      "actions",
    ];
  }

  public openReport(report: Report): void {
    this.router.navigate([
      "/reports",
      "group",
      this.groupId,
      "report",
      report.id,
    ]);
  }

  public openAddDialog(): void {
    const dialogRef = this.matDialog.open(
      ReportFormComponent,
      DEFAULT_DIALOG_CONFIG
    );

    dialogRef.componentInstance.groupId = this.groupId;
    dialogRef.componentInstance.headerText = "Add report";

    dialogRef
      .afterClosed()
      .pipe(
        take(1),
        tap((refreshData) => {
          if (refreshData) {
            this.getReports();
          }
        })
      )
      .subscribe();
  }

  public openDeleteConfirmationDialog(report: Report): void {
    const dialogRef = this.matDialog.open(
      ConfirmationDialogComponent,
      DEFAULT_DIALOG_CONFIG
    );

    dialogRef.componentInstance.headerText = `Delete ${report.name}`;
    dialogRef.componentInstance.dialogContent = `Are you sure you want to delete ${report.name}? This will not delete the receipts assigned to it.`;

    dialogRef
      .afterClosed()
      .pipe(
        take(1),
        switchMap((confirmed) => {
          if (confirmed) {
            return this.reportService.deleteReport(report.id).pipe(
              tap(() => {
                this.snackbarService.success("Report successfully deleted");
                this.store.dispatch(new RemoveReport(report.id));
                this.dataSource.set(
                  new MatTableDataSource<Report>(this.reports())
                );
              })
            );
          } else {
            return of(undefined);
          }
        })
      )
      .subscribe();
  }
}
