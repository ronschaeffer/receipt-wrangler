import { Component, Input, OnInit } from "@angular/core";
import { FormBuilder, FormGroup, Validators } from "@angular/forms";
import { MatDialogRef } from "@angular/material/dialog";
import { take, tap } from "rxjs";
import { FormOption } from "src/interfaces/form-option.interface";
import { Report, ReportService } from "../../open-api";

@Component({
  selector: "app-add-to-report-dialog",
  templateUrl: "./add-to-report-dialog.component.html",
  styleUrls: ["./add-to-report-dialog.component.scss"],
  standalone: false,
})
export class AddToReportDialogComponent implements OnInit {
  @Input() public groupId!: string;

  public form: FormGroup = new FormGroup({});

  public reportOptions: FormOption[] = [];

  public loading: boolean = true;

  constructor(
    private formBuilder: FormBuilder,
    public matDialogRef: MatDialogRef<AddToReportDialogComponent>,
    private reportService: ReportService
  ) {}

  public ngOnInit(): void {
    this.initForm();
    this.loadReports();
  }

  private initForm(): void {
    this.form = this.formBuilder.group({
      reportId: [null, Validators.required],
    });
  }

  private loadReports(): void {
    this.reportService
      .getReportsForGroup(this.groupId)
      .pipe(
        take(1),
        tap((reports: Report[]) => {
          this.reportOptions = reports.map((report) => ({
            value: report.id,
            displayValue: report.name,
          }));
          this.loading = false;
        })
      )
      .subscribe();
  }

  public cancelButtonClicked(): void {
    this.matDialogRef.close(undefined);
  }

  public submitButtonClicked(): void {
    if (this.form.valid) {
      this.matDialogRef.close(this.form.value.reportId);
    }
  }
}
