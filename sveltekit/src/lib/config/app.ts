export const APP_NAME = 'Xemu Cartographer';

export type OAuthProviderMeta = {
	label: string;
	icon: string;
};

export const OAUTH_PROVIDERS: Record<string, OAuthProviderMeta> = {
	discord: { label: 'Discord', icon: '/oauth/discord.svg' },
	twitch: { label: 'Twitch', icon: '/oauth/twitch.svg' },
	google: { label: 'Google', icon: '/oauth/google.svg' },
	microsoft: { label: 'Microsoft', icon: '/oauth/microsoft.svg' },
	github: { label: 'GitHub', icon: '/oauth/github.svg' },
	apple: { label: 'Apple', icon: '/oauth/apple.svg' },
	facebook: { label: 'Facebook', icon: '/oauth/facebook.svg' },
	spotify: { label: 'Spotify', icon: '/oauth/spotify.svg' },
	patreon: { label: 'Patreon', icon: '/oauth/patreon.svg' },
	instagram: { label: 'Instagram', icon: '/oauth/instagram.svg' }
};
