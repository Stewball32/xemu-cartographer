import { createToaster } from '@skeletonlabs/skeleton-svelte';
import { AdminFetchError } from '$lib/utils/admin-api';

export const toaster = createToaster({
	placement: 'bottom-end',
	duration: 4000
});

export function describeAsyncError(err: unknown): string {
	if (err instanceof AdminFetchError) return err.message;
	if (err instanceof Error) return err.message;
	return String(err);
}

type ToastConfig = { title: string; description?: string };

type ToastLabels<T = unknown> = {
	loading: ToastConfig;
	success: ToastConfig | ((result: T) => ToastConfig);
	errorTitle: string;
	/** Override the error description (default: `describeAsyncError(err)`). */
	errorDescription?: (err: unknown) => string;
};

export function toastPromise<T>(promise: Promise<T>, labels: ToastLabels<T>): Promise<T> {
	toaster.promise(promise, {
		loading: labels.loading,
		success: (result) =>
			typeof labels.success === 'function' ? labels.success(result) : labels.success,
		error: (err) => ({
			title: labels.errorTitle,
			description: (labels.errorDescription ?? describeAsyncError)(err)
		})
	});
	return promise;
}

export type ToastConfirmMeta = {
	confirmLabel: string;
	cancelLabel: string;
	onConfirm: () => void;
	onCancel: () => void;
};

type ConfirmOptions = {
	title: string;
	description?: string;
	confirmLabel?: string;
	cancelLabel?: string;
	type?: 'info' | 'warning' | 'error';
	/** Duration in ms before auto-cancel. Default 60000 (1 minute). */
	duration?: number;
};

/**
 * Show a non-blocking confirmation toast with Confirm/Cancel buttons.
 * Resolves true if Confirm is clicked; false if Cancel/close/timeout.
 */
export function confirmToast(opts: ConfirmOptions): Promise<boolean> {
	return new Promise<boolean>((resolve) => {
		let settled = false;
		let toastId = '';
		const settle = (v: boolean) => {
			if (settled) return;
			settled = true;
			if (toastId) toaster.dismiss(toastId);
			resolve(v);
		};

		const confirm: ToastConfirmMeta = {
			confirmLabel: opts.confirmLabel ?? 'Confirm',
			cancelLabel: opts.cancelLabel ?? 'Cancel',
			onConfirm: () => settle(true),
			onCancel: () => settle(false)
		};

		toastId = toaster.create({
			type: opts.type ?? 'warning',
			title: opts.title,
			description: opts.description,
			duration: opts.duration ?? 60000,
			closable: true,
			meta: { confirm },
			onStatusChange: (details) => {
				// Auto-dismiss (timeout) or X-button close — treat as cancel.
				if (details.status === 'dismissing' || details.status === 'unmounted') {
					settle(false);
				}
			}
		});
	});
}
