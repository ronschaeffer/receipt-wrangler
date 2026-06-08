import { Component, OnInit } from "@angular/core";
import { AbstractControl, FormArray, FormBuilder, FormGroup } from "@angular/forms";
import { ActivatedRoute, Router } from "@angular/router";
import { Store } from "@ngxs/store";
import { switchMap, take, tap } from "rxjs";
import { FormMode } from "../../enums/form-mode.enum";
import { BaseFormComponent } from "../../form/index";
import { Group, GroupRole, GroupsService, GroupTaxRule } from "../../open-api/index";
import { SnackbarService } from "../../services/index";
import { UpdateGroup } from "../../store/index";
import { GroupUtil } from "../../utils/index";

@Component({
    selector: "app-group-receipt-settings",
    templateUrl: "./group-receipt-settings.component.html",
    styleUrl: "./group-receipt-settings.component.scss",
    standalone: false
})
export class GroupReceiptSettingsComponent extends BaseFormComponent implements OnInit {
  public originalGroup!: Group;

  public editLink: string = "";

  public canEdit = false;

  constructor(
    private activatedRoute: ActivatedRoute,
    private formBuilder: FormBuilder,
    private groupUtil: GroupUtil,
    private groupsService: GroupsService,
    private router: Router,
    private snackbarService: SnackbarService,
    private store: Store,
  ) {
    super();
  }

  public ngOnInit(): void {
    this.setFormConfigFromRoute(this.activatedRoute);
    this.setOriginalGroup();
    this.initForm();
    this.canEdit = this.groupUtil.hasGroupAccess(this.originalGroup.id, GroupRole.Owner, false, false);
  }

  private initForm(): void {
    const receiptSettings = this.originalGroup.groupReceiptSettings;
    this.form = this.formBuilder.group({
      hideImages: [receiptSettings.hideImages ?? false],
      hideReceiptCategories: [receiptSettings.hideReceiptCategories ?? false],
      hideReceiptTags: [receiptSettings.hideReceiptTags ?? false],
      hideItemCategories: [receiptSettings.hideItemCategories ?? false],
      hideItemTags: [receiptSettings.hideItemTags ?? false],
      hideShareCategories: [receiptSettings.hideShareCategories ?? false],
      hideShareTags: [receiptSettings.hideShareTags ?? false],
      hideComments: [receiptSettings.hideComments ?? false],
      homeCurrency: [receiptSettings.homeCurrency ?? "GBP"],
      usePrintedTax: [receiptSettings.usePrintedTax ?? true],
      defaultTaxRate: [receiptSettings.defaultTaxRate ?? null],
      taxRules: this.formBuilder.array(
        (receiptSettings.taxRules ?? []).map((rule) => this.buildTaxRuleFormGroup(rule))
      ),
    });

    if (this.formConfig.mode != FormMode.edit) {
      this.form.disable();
    }
  }

  private setOriginalGroup(): void {
    this.originalGroup = this.activatedRoute.snapshot.data["group"];
    this.editLink = `/groups/${this.originalGroup.id}/receipt-settings/edit`;
  }

  public get taxRulesFormArray(): FormArray {
    return this.form.get("taxRules") as FormArray;
  }

  public asFormGroup(control: AbstractControl): FormGroup {
    return control as FormGroup;
  }

  private buildTaxRuleFormGroup(rule?: GroupTaxRule): FormGroup {
    return this.formBuilder.group({
      countryCode: [rule?.countryCode ?? ""],
      reclaimable: [rule?.reclaimable ?? false],
      requireTaxId: [rule?.requireTaxId ?? false],
      defaultRate: [rule?.defaultRate ?? null],
      label: [rule?.label ?? ""],
    });
  }

  public addTaxRule(): void {
    this.taxRulesFormArray.push(this.buildTaxRuleFormGroup());
  }

  public removeTaxRule(index: number): void {
    this.taxRulesFormArray.removeAt(index);
  }

  public submit(): void {
    if (this.form.valid) {
      this.groupsService.updateGroupReceiptSettings(this.originalGroup.id,
        this.form.value)
        .pipe(
          take(1),
          switchMap((updatedGroupReceiptSettings) => {
            this.originalGroup.groupReceiptSettings = updatedGroupReceiptSettings;
            return this.store.dispatch(new UpdateGroup(this.originalGroup));
          }),
          tap(() => {
            this.snackbarService.success("Receipt settings updated successfully");
            this.router.navigate(
              [`/groups/${this.originalGroup.id}/receipt-settings/view`],
              {
                queryParams: {
                  tab: "receipt-settings"
                }
              }
            );
          })
        )
        .subscribe();
    }
  }
}
