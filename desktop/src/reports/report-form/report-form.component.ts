import { Component, Input, OnInit } from "@angular/core";
import { FormBuilder, FormGroup, Validators } from "@angular/forms";
import { MatDialogRef } from "@angular/material/dialog";
import { take, tap } from "rxjs";
import { Report, ReportService, UpsertReportCommand } from "../../open-api";
import { SnackbarService } from "../../services";

@Component({
  selector: "app-report-form",
  templateUrl: "./report-form.component.html",
  styleUrls: ["./report-form.component.scss"],
  standalone: false,
})
export class ReportFormComponent implements OnInit {
  @Input() public headerText: string = "";

  @Input() public report?: Report;

  @Input() public groupId!: string;

  public form: FormGroup = new FormGroup({});

  constructor(
    private formBuilder: FormBuilder,
    private matDialogRef: MatDialogRef<ReportFormComponent>,
    private reportService: ReportService,
    private snackService: SnackbarService
  ) {}

  public ngOnInit(): void {
    this.initForm();
  }

  private initForm(): void {
    this.form = this.formBuilder.group({
      name: [this.report?.name ?? "", Validators.required],
    });
  }

  public submit(): void {
    if (!this.form.valid) {
      return;
    }

    if (this.report) {
      const command: UpsertReportCommand = {
        name: this.form.value.name,
        groupId: this.report.groupId,
        status: this.report.status,
      };
      this.reportService
        .updateReport(this.report.id, command)
        .pipe(
          take(1),
          tap(() => {
            this.snackService.success("Report updated successfully");
            this.matDialogRef.close(true);
          })
        )
        .subscribe();
    } else {
      const command: UpsertReportCommand = {
        name: this.form.value.name,
        groupId: Number(this.groupId),
      };
      this.reportService
        .createReport(command)
        .pipe(
          take(1),
          tap(() => {
            this.snackService.success("Report created successfully");
            this.matDialogRef.close(true);
          })
        )
        .subscribe();
    }
  }

  public closeDialog(): void {
    this.matDialogRef.close(false);
  }
}
