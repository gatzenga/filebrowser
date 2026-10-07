const name: string = window.FileBrowser.Name || "File Browser";
const disableExternal: boolean = window.FileBrowser.DisableExternal;
const disableUsedPercentage: boolean = window.FileBrowser.DisableUsedPercentage;
const baseURL: string = window.FileBrowser.BaseURL;
const staticURL: string = window.FileBrowser.StaticURL;
const recaptcha: string = window.FileBrowser.ReCaptcha;
const recaptchaKey: string = window.FileBrowser.ReCaptchaKey;
const version: string = window.FileBrowser.Version;
const logoURL = `${staticURL}/img/logo.svg`;
const noAuth: boolean = window.FileBrowser.NoAuth;
const authMethod = window.FileBrowser.AuthMethod;
const logoutPage: string = window.FileBrowser.LogoutPage;
const loginPage: boolean = window.FileBrowser.LoginPage;
const theme: UserTheme = window.FileBrowser.Theme;
const enableThumbs: boolean = window.FileBrowser.EnableThumbs;
const resizePreview: boolean = window.FileBrowser.ResizePreview;
const enableExec: boolean = window.FileBrowser.EnableExec;
const origin = window.location.origin;

export {
  name,
  disableExternal,
  disableUsedPercentage,
  baseURL,
  logoURL,
  recaptcha,
  recaptchaKey,
  version,
  noAuth,
  authMethod,
  logoutPage,
  loginPage,
  theme,
  enableThumbs,
  resizePreview,
  enableExec,
  origin,
};
