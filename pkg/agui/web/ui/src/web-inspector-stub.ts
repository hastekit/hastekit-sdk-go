// Stub for @copilotkit/web-inspector (~850KB). The real package is
// imported only by CopilotKit's CopilotKitInspector, which mounts only
// when showDevConsole is true — we always pass false, so this code never
// runs at runtime. Aliasing it here keeps it out of the embedded bundle.
export const WEB_INSPECTOR_TAG = "copilotkit-web-inspector-disabled";
export class WebInspectorElement {}
export function defineWebInspector(): void {}
