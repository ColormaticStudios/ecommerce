export const adminSurfaceVariantClasses = {
	panel:
		"rounded-[1.25rem] border border-stone-200/90 bg-stone-50/80 p-6 shadow-[0_24px_80px_-48px_rgba(41,37,36,0.35)] dark:border-stone-800 dark:bg-stone-950/80",
	"panel-tight":
		"rounded-[1.25rem] border border-stone-200/90 bg-stone-50/80 p-5 shadow-[0_24px_80px_-48px_rgba(41,37,36,0.35)] dark:border-stone-800 dark:bg-stone-950/80",
	subsurface:
		"rounded-2xl border border-stone-200/90 bg-white/85 p-4 shadow-sm dark:border-stone-800 dark:bg-stone-950/80",
	muted:
		"rounded-2xl border border-stone-200/80 bg-stone-100/70 p-4 dark:border-stone-800 dark:bg-stone-900/60",
	media:
		"rounded-2xl border border-stone-200/90 bg-white/85 p-0 shadow-sm dark:border-stone-800 dark:bg-stone-950/80",
} as const;

export type AdminSurfaceVariant = keyof typeof adminSurfaceVariantClasses;

export const adminDividerTopClass = "border-t border-stone-200 pt-6 dark:border-stone-800";
export const adminDividerBottomClass = "border-b border-stone-200 pb-6 dark:border-stone-800";

export const adminListItemBaseClass =
	"rounded-2xl border border-stone-200/90 bg-white/90 shadow-sm transition dark:border-stone-800 dark:bg-stone-950/80";
export const adminListItemActiveClass =
	"border-stone-900 bg-stone-100 shadow-[0_16px_50px_-36px_rgba(28,25,23,0.65)] dark:border-stone-100 dark:bg-stone-900";
export const adminListItemInteractiveClass =
	"cursor-pointer hover:border-stone-300 hover:bg-stone-50 dark:hover:border-stone-700 dark:hover:bg-stone-900";
