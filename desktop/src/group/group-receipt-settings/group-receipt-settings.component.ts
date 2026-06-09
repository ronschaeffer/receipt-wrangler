import { Component, OnInit } from "@angular/core";
import { AbstractControl, FormArray, FormBuilder, FormGroup } from "@angular/forms";
import { ActivatedRoute, Router } from "@angular/router";
import { Store } from "@ngxs/store";
import { forkJoin, switchMap, take, tap } from "rxjs";
import { FormMode } from "../../enums/form-mode.enum";
import { BaseFormComponent } from "../../form/index";
import { Category, CategoryService, CustomField, CustomFieldService, CustomFieldType, Group, GroupRole, GroupsService, GroupTaxRule } from "../../open-api/index";
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

  // #1 VAT/currency via custom fields: candidate custom fields for the dropdowns.
  public vatCustomFieldOptions: CustomField[] = [];
  public currencyCustomFieldOptions: CustomField[] = [];

  // #5 group-scoped categories: all categories + the group's enabled-id set.
  public allCategories: Category[] = [];
  public enabledCategoryIds = new Set<number>();
  public savingCategories = false;

  constructor(
    private activatedRoute: ActivatedRoute,
    private categoryService: CategoryService,
    private customFieldService: CustomFieldService,
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
    this.loadCustomFieldOptions();
    this.loadCategoryConfiguration();
  }

  // Fetch all custom fields and split into VAT (CURRENCY) and currency
  // (SELECT or TEXT) candidates for the report-mapping dropdowns.
  private loadCustomFieldOptions(): void {
    this.customFieldService
      .getPagedCustomFields({ page: 1, pageSize: 1000 })
      .pipe(take(1))
      .subscribe((paged) => {
        const fields = (paged?.data ?? []) as CustomField[];
        this.vatCustomFieldOptions = fields.filter((f) => f.type === CustomFieldType.Currency);
        this.currencyCustomFieldOptions = fields.filter(
          (f) => f.type === CustomFieldType.Select || f.type === CustomFieldType.Text
        );
      });
  }

  // Load the full category list and the group's currently-enabled set.
  private loadCategoryConfiguration(): void {
    forkJoin({
      all: this.categoryService.getAllCategories().pipe(take(1)),
      enabled: this.groupsService.getGroupCategories(this.originalGroup.id).pipe(take(1)),
    }).subscribe(({ all, enabled }) => {
      this.allCategories = all ?? [];
      // If the group has no explicit subset, the API returns all categories
      // (the fallback). Treat "enabled == all" as "none configured" so the UI
      // shows an unticked list the owner can opt into, rather than everything
      // ticked. We detect this by comparing counts.
      const enabledList = enabled ?? [];
      if (enabledList.length === this.allCategories.length) {
        this.enabledCategoryIds = new Set<number>();
      } else {
        this.enabledCategoryIds = new Set<number>(
          enabledList.map((c) => c.id).filter((id): id is number => id != null)
        );
      }
    });
  }

  public isCategoryEnabled(categoryId?: number): boolean {
    return categoryId != null && this.enabledCategoryIds.has(categoryId);
  }

  public toggleCategory(categoryId?: number): void {
    if (categoryId == null) {
      return;
    }
    if (this.enabledCategoryIds.has(categoryId)) {
      this.enabledCategoryIds.delete(categoryId);
    } else {
      this.enabledCategoryIds.add(categoryId);
    }
  }

  // Persist the group's enabled-category set via the dedicated endpoint
  // (separate from the receipt-settings form submit).
  public saveGroupCategories(): void {
    this.savingCategories = true;
    this.groupsService
      .setGroupCategories(this.originalGroup.id, { categoryIds: Array.from(this.enabledCategoryIds) })
      .pipe(take(1))
      .subscribe({
        next: () => {
          this.savingCategories = false;
          this.snackbarService.success("Group categories updated");
        },
        error: () => {
          this.savingCategories = false;
          this.snackbarService.error("Failed to update group categories");
        },
      });
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
      vatCustomFieldId: [receiptSettings.vatCustomFieldId ?? null],
      currencyCustomFieldId: [receiptSettings.currencyCustomFieldId ?? null],
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

  public get currentTemplateName(): string {
    return this.originalGroup.groupReceiptSettings?.reportTemplateName ?? "";
  }

  public onTemplateSelected(event: Event): void {
    const input = event.target as HTMLInputElement;
    const file = input.files?.[0];
    if (!file) {
      return;
    }
    this.groupsService
      .uploadReportTemplate(this.originalGroup.id, file)
      .pipe(
        take(1),
        switchMap((updated) => {
          this.originalGroup.groupReceiptSettings = updated;
          return this.store.dispatch(new UpdateGroup(this.originalGroup));
        }),
        tap(() => this.snackbarService.success("Custom report template uploaded"))
      )
      .subscribe();
    input.value = "";
  }

  public removeTemplate(): void {
    this.groupsService
      .deleteReportTemplate(this.originalGroup.id)
      .pipe(
        take(1),
        switchMap((updated) => {
          this.originalGroup.groupReceiptSettings = updated;
          return this.store.dispatch(new UpdateGroup(this.originalGroup));
        }),
        tap(() => this.snackbarService.success("Custom report template removed"))
      )
      .subscribe();
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
