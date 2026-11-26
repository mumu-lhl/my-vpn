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

/**
 * Builds the unified app catalog shown in the Per-App Proxy selector:
 * - Includes all installed apps returned by Android (excluding Looma itself).
 * - Preserves any manually-selected or preset packages not present in the list
 *   so users can always inspect and uncheck them.
 * - Annotates detected China apps with their detection reason.
 */
export function buildPerAppCatalog(
  installedApps: InstalledApp[],
  selectedPackages: string[],
  detectedList?: DetectedCNApp[],
): PerAppEntry[] {
  const selectedSet = new Set(selectedPackages);
  const classicMap = new Map(
    CLASSIC_CN_APPS.map((a) => [a.package_name.toLowerCase(), a]),
  );

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

  for (const rawPkg of selectedPackages) {
    const pkg = rawPkg.trim();
    if (!pkg || SELF_PACKAGES.has(pkg.toLowerCase()) || byPkg.has(pkg)) {
      continue;
    }
    const classic = classicMap.get(pkg.toLowerCase());
    byPkg.set(pkg, {
      package_name: pkg,
      label: classic ? classic.name : pkg,
      is_system: false,
    });
  }

  const allApps = Array.from(byPkg.values());
  const detected = detectedList ?? detectChinaApps(allApps);
  const detectedMap = new Map(
    detected.map((d) => [d.package_name.toLowerCase(), d.reason]),
  );

  return allApps.map((app) => {
    const key = app.package_name.toLowerCase();
    const classic = classicMap.get(key);
    return {
      package_name: app.package_name,
      label: app.label || classic?.name || app.package_name,
      is_system: Boolean(app.is_system),
      installer_package: app.installer_package,
      selected: selectedSet.has(app.package_name),
      cn_reason: detectedMap.get(key),
      category: classic?.category,
    };
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
    return a.label.localeCompare(b.label, "zh-Hans-CN", {
      sensitivity: "base",
    });
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
