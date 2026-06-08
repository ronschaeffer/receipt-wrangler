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
import { ActivatedRoute } from "@angular/router";
import { Store } from "@ngxs/store";
import { of, switchMap, take, tap } from "rxjs";
import { DEFAULT_DIALOG_CONFIG } from "src/constants";
import { ConfirmationDialogComponent } from "src/shared-ui/confirmation-dialog/confirmation-dialog.component";
import { TableColumn } from "src/table/table-column.interface";
import {
  BulkReportReceiptCommand,
  Receipt,
  Report,
  ReportService,
  ReportStatus,
  UpsertReportCommand,
} from "../../open-api";
import { SnackbarService } from "../../services";
import { UpdateReport } from "../../store/report.state.actions";
import { downloadFile } from "../../utils/file";

@Component({
  selector: "app-report-detail",
  templateUrl: "./report-detail.component.html",
  styleUrls: ["./report-detail.component.scss"],
  standalone: false,
})
export class ReportDetailComponent implements OnInit, AfterViewInit {
  public readonly nameCell = viewChild.required<TemplateRef<any>>("nameCell");

  public readonly amountCell =
    viewChild.required<TemplateRef<any>>("amountCell");

  public readonly dateCell = viewChild.required<TemplateRef<any>>("dateCell");

  public readonly actionsCell =
    viewChild.required<TemplateRef<any>>("actionsCell");

  public report = signal<Report | undefined>(undefined);

  public dataSource = signal(new MatTableDataSource<Receipt>([]));

  public displayedColumns: string[] = [];

  public columns: TableColumn[] = [];

  public reportStatus = ReportStatus;

  public groupId!: string;

  public reportId!: number;

  public backLink: string[] = [];

  constructor(
    private reportService: ReportService,
    private snackbarService: SnackbarService,
    private store: Store,
    private activatedRoute: ActivatedRoute,
    private matDialog: MatDialog
  ) {}

  public ngOnInit(): void {
    this.groupId = this.activatedRoute.snapshot.params["groupId"];
    this.reportId = Number(this.activatedRoute.snapshot.params["reportId"]);
    this.backLink = ["/reports", "group", this.groupId];
    this.getReport();
  }

  public ngAfterViewInit(): void {
    this.setColumns();
  }

  private getReport(): void {
    this.reportService
      .getReport(this.reportId)
      .pipe(
        take(1),
        tap((report) => {
          this.report.set(report);
          this.dataSource.set(
            new MatTableDataSource<Receipt>(report.receipts ?? [])
          );
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
        columnHeader: "Amount",
        matColumnDef: "amount",
        template: this.amountCell(),
        sortable: false,
      },
      {
        columnHeader: "Date",
        matColumnDef: "date",
        template: this.dateCell(),
        sortable: false,
      },
      {
        columnHeader: "Actions",
        matColumnDef: "actions",
        template: this.actionsCell(),
        sortable: false,
      },
    ] as TableColumn[];

    this.displayedColumns = ["name", "amount", "date", "actions"];
  }

  public updateStatus(status: ReportStatus): void {
    const report = this.report();
    if (!report) {
      return;
    }

    const command: UpsertReportCommand = {
      name: report.name,
      groupId: report.groupId,
      status: status,
    };

    this.reportService
      .updateReport(report.id, command)
      .pipe(
        take(1),
        tap((updated) => {
          this.report.set(updated);
          this.store.dispatch(new UpdateReport(updated));
          this.snackbarService.success("Report updated successfully");
        })
      )
      .subscribe();
  }

  public removeReceipt(receipt: Receipt): void {
    const dialogRef = this.matDialog.open(
      ConfirmationDialogComponent,
      DEFAULT_DIALOG_CONFIG
    );

    dialogRef.componentInstance.headerText = "Remove receipt";
    dialogRef.componentInstance.dialogContent = `Remove ${receipt.name} from this report? The receipt itself will not be deleted.`;

    dialogRef
      .afterClosed()
      .pipe(
        take(1),
        switchMap((confirmed) => {
          if (!confirmed) {
            return of(undefined);
          }

          const command: BulkReportReceiptCommand = {
            receiptIds: [receipt.id as number],
          };

          return this.reportService
            .removeReceiptsFromReport(this.reportId, command)
            .pipe(
              take(1),
              tap((report) => {
                this.report.set(report);
                this.store.dispatch(new UpdateReport(report));
                this.dataSource.set(
                  new MatTableDataSource<Receipt>(report.receipts ?? [])
                );
                this.snackbarService.success("Receipt removed from report");
              })
            );
        })
      )
      .subscribe();
  }

  public exportCsv(): void {
    this.reportService
      .exportReportCsv(this.reportId)
      .pipe(
        take(1),
        tap((blob) => {
          const name = this.report()?.name ?? "report";
          downloadFile(blob, `${name}-receipts.zip`);
        })
      )
      .subscribe();
  }

  public exportPack(): void {
    this.reportService
      .exportReportReceiptPack(this.reportId)
      .pipe(
        take(1),
        tap((blob) => {
          const name = this.report()?.name ?? "report";
          downloadFile(blob, `${name}-pack.pdf`);
        })
      )
      .subscribe();
  }
}
