import {
  CLASSIC_CN_APPS,
  detectChinaApps,
  mergeUniquePackages,
  type DetectedCNApp,
  type InstalledApp,
} from "./cn_apps";

export interface PerAppEntry {
  package_name: string;
  label: string;
  is_system: boolean;
  installer_package?: string;
  selected: boolean;
  cn_reason?: string;
  category?: string;
}

export interface PerAppBaseEntry {
  package_name: string;
  label: string;
  label_lower: string;
  pkg_lower: string;
  category_lower: string;
  is_system: boolean;
  installer_package?: string;
  cn_reason?: string;
  category?: string;
}

export interface PerAppFilterOptions {
  query: string;
  showSystem: boolean;
  onlySelected: boolean;
  selectedFirst: boolean;
}

const SELF_PACKAGES = new Set([
  "io.github.mumulhl.looma",
  "io.github.mumulhl.myvpn",
]);

const CLASSIC_MAP = new Map(
  CLASSIC_CN_APPS.map((a) => [a.package_name.toLowerCase(), a]),
);

const ZH_COLLATOR = new Intl.Collator("zh-Hans-CN", { sensitivity: "base" });

/**
 * Precomputes the static sorted app catalog (including China app detection and lowercased
 * search keys) so toggling checkboxes or typing in the search bar does not rerun detection
 * or re-sort 300+ apps via localeCompare on every keystroke.
 */
export function buildPerAppBaseCatalog(
  installedApps: InstalledApp[],
  detectedList?: DetectedCNApp[],
): PerAppBaseEntry[] {
  const byPkg = new Map<string, InstalledApp>();
  for (const app of installedApps) {
    const pkg = app.package_name.trim();
    if (!pkg || SELF_PACKAGES.has(pkg.toLowerCase())) {
      continue;
    }
    byPkg.set(pkg, {
      package_name: pkg,
      label: app.label?.trim() || pkg,
      is_system: Boolean(app.is_system),
      installer_package: app.installer_package,
    });
  }

  const allApps = Array.from(byPkg.values());
  const detected = detectedList ?? detectChinaApps(allApps);
  const detectedMap = new Map(
    detected.map((d) => [d.package_name.toLowerCase(), d.reason]),
  );

  const entries: PerAppBaseEntry[] = allApps.map((app) => {
    const pkgLower = app.package_name.toLowerCase();
    const classic = CLASSIC_MAP.get(pkgLower);
    const label = app.label || classic?.name || app.package_name;
    const category = classic?.category;
    return {
      package_name: app.package_name,
      label,
      label_lower: label.toLowerCase(),
      pkg_lower: pkgLower,
      category_lower: category ? category.toLowerCase() : "",
      is_system: Boolean(app.is_system),
      installer_package: app.installer_package,
      cn_reason: detectedMap.get(pkgLower),
      category,
    };
  });

  entries.sort((a, b) => ZH_COLLATOR.compare(a.label, b.label));
  return entries;
}

/**
 * Fast O(N) filter pass over a pre-sorted base catalog that merges manually-added
 * selected packages and partitions selected items first without re-sorting.
 */
export function filterPerAppBaseCatalog(
  base: PerAppBaseEntry[],
  selectedPackages: string[],
  opts: PerAppFilterOptions,
): PerAppEntry[] {
  const selectedSet = new Set(selectedPackages);
  const q = opts.query.trim().toLowerCase();

  const selectedOut: PerAppEntry[] = [];
  const unselectedOut: PerAppEntry[] = [];
  const seen = new Set<string>();

  for (const item of base) {
    seen.add(item.package_name);
    const selected = selectedSet.has(item.package_name);
    if (!opts.showSystem && item.is_system && !selected) {
      continue;
    }
    if (opts.onlySelected && !selected) {
      continue;
    }
    if (
      q &&
      !item.label_lower.includes(q) &&
      !item.pkg_lower.includes(q) &&
      !item.category_lower.includes(q)
    ) {
      continue;
    }
    const entry: PerAppEntry = {
      package_name: item.package_name,
      label: item.label,
      is_system: item.is_system,
      installer_package: item.installer_package,
      selected,
      cn_reason: item.cn_reason,
      category: item.category,
    };
    if (opts.selectedFirst && selected) {
      selectedOut.push(entry);
    } else {
      unselectedOut.push(entry);
    }
  }

  // Append manually added / preset packages not found in installedApps.
  for (const rawPkg of selectedPackages) {
    const pkg = rawPkg.trim();
    if (!pkg || seen.has(pkg) || SELF_PACKAGES.has(pkg.toLowerCase())) {
      continue;
    }
    const pkgLower = pkg.toLowerCase();
    const classic = CLASSIC_MAP.get(pkgLower);
    const label = classic ? classic.name : pkg;
    const category = classic?.category;
    if (
      q &&
      !label.toLowerCase().includes(q) &&
      !pkgLower.includes(q) &&
      !(category?.toLowerCase().includes(q) ?? false)
    ) {
      continue;
    }
    const entry: PerAppEntry = {
      package_name: pkg,
      label,
      is_system: false,
      selected: true,
      category,
    };
    if (opts.selectedFirst) {
      selectedOut.push(entry);
    } else {
      unselectedOut.push(entry);
    }
  }

  return opts.selectedFirst
    ? [...selectedOut, ...unselectedOut]
    : unselectedOut;
}

/**
 * Legacy helper preserved for existing callers and unit tests.
 */
export function buildPerAppCatalog(
  installedApps: InstalledApp[],
  selectedPackages: string[],
  detectedList?: DetectedCNApp[],
): PerAppEntry[] {
  const base = buildPerAppBaseCatalog(installedApps, detectedList);
  return filterPerAppBaseCatalog(base, selectedPackages, {
    query: "",
    showSystem: true,
    onlySelected: false,
    selectedFirst: false,
  });
}

export function filterAndSortPerApps(
  entries: PerAppEntry[],
  opts: PerAppFilterOptions,
): PerAppEntry[] {
  const q = opts.query.trim().toLowerCase();
  const filtered = entries.filter((item) => {
    if (!opts.showSystem && item.is_system && !item.selected) {
      return false;
    }
    if (opts.onlySelected && !item.selected) {
      return false;
    }
    if (q) {
      const hitLabel = item.label.toLowerCase().includes(q);
      const hitPkg = item.package_name.toLowerCase().includes(q);
      const hitCat = item.category?.toLowerCase().includes(q) ?? false;
      if (!hitLabel && !hitPkg && !hitCat) {
        return false;
      }
    }
    return true;
  });

  return filtered.sort((a, b) => {
    if (opts.selectedFirst && a.selected !== b.selected) {
      return a.selected ? -1 : 1;
    }
    return ZH_COLLATOR.compare(a.label, b.label);
  });
}

export function togglePackageSelection(
  selected: string[],
  pkg: string,
): string[] {
  if (selected.includes(pkg)) {
    return selected.filter((item) => item !== pkg);
  }
  return [...selected, pkg];
}

export function selectClassicCNPackages(
  installedApps: InstalledApp[],
  selected: string[],
): { packages: string[]; addedCount: number } {
  const installedSet = new Set(
    installedApps.map((a) => a.package_name.toLowerCase()),
  );
  const candidates =
    installedApps.length > 0
      ? CLASSIC_CN_APPS.filter((a) =>
          installedSet.has(a.package_name.toLowerCase()),
        ).map((a) => a.package_name)
      : CLASSIC_CN_APPS.map((a) => a.package_name);
  const next = mergeUniquePackages(selected, candidates);
  return {
    packages: next,
    addedCount: next.length - selected.length,
  };
}

export function selectDetectedCNPackages(
  installedApps: InstalledApp[],
  selected: string[],
  detected?: DetectedCNApp[],
): { packages: string[]; addedCount: number; detectedCount: number } {
  const hits = detected ?? detectChinaApps(installedApps);
  const next = mergeUniquePackages(
    selected,
    hits.map((d) => d.package_name),
  );
  return {
    packages: next,
    addedCount: next.length - selected.length,
    detectedCount: hits.length,
  };
}

export function fallbackPreviewInstalledApps(): InstalledApp[] {
  return CLASSIC_CN_APPS.map((a) => ({
    package_name: a.package_name,
    label: a.name,
    is_system: false,
  }));
}
