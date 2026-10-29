export const styles = `
.scheduled-dnd { color: var(--center-channel-color, #3d3c40); background: var(--center-channel-bg, #fff); border: 1px solid rgba(var(--center-channel-color-rgb, 63,67,80), .2); border-radius: 12px; width: min(680px, calc(100vw - 32px)); max-height: calc(100vh - 48px); padding: 28px; box-shadow: 0 16px 48px #0004; }
.scheduled-dnd::backdrop { background: #0007; }
.scheduled-dnd header { display: flex; justify-content: space-between; align-items: start; gap: 16px; }
.scheduled-dnd h2 { margin: 6px 0 12px; font-size: 24px; font-weight: 600; }
.scheduled-dnd h3 { font-size: 16px; font-weight: 600; }
.scheduled-dnd p { line-height: 1.5; }
.scheduled-dnd .dnd-eyebrow { font-size: 11px; letter-spacing: 1px; opacity: .7; }
.scheduled-dnd .dnd-close { border: 0; color: inherit; background: transparent; font-size: 28px; line-height: 1; padding: 4px; }
.scheduled-dnd .dnd-status { border-left: 3px solid var(--button-bg, #166de0); background: rgba(var(--center-channel-color-rgb, 63,67,80), .04); padding: 16px; margin: 20px 0; line-height: 1.7; }
.scheduled-dnd .dnd-fields { border: 0; padding: 0; min-width: 0; display: grid; gap: 12px; }
.scheduled-dnd .dnd-toggle { display: flex; align-items: center; gap: 8px; padding: 8px 0; font-weight: 600; }
.scheduled-dnd input[type=checkbox] { accent-color: var(--button-bg, #166de0); }
.scheduled-dnd .form-control { color: inherit; background: var(--center-channel-bg, #fff); border: 1px solid rgba(var(--center-channel-color-rgb, 63,67,80), .3); border-radius: 4px; min-height: 40px; padding: 8px 12px; width: 100%; }
.scheduled-dnd .dnd-rule { border: 1px solid rgba(var(--center-channel-color-rgb, 63,67,80), .2); border-radius: 8px; padding: 16px; margin: 6px 0; min-width: 0; }
.scheduled-dnd legend { width: auto; color: inherit; border: 0; font-size: 14px; font-weight: 600; margin: 0; padding: 0 6px; }
.scheduled-dnd .dnd-days { display: flex; flex-wrap: wrap; gap: 12px; margin-bottom: 12px; text-transform: capitalize; }
.scheduled-dnd .dnd-days label { display: flex; align-items: center; gap: 5px; }
.scheduled-dnd .dnd-times { display: flex; flex-wrap: wrap; gap: 12px; align-items: end; }
.scheduled-dnd .dnd-times label { flex: 1; min-width: 120px; }
.scheduled-dnd small, .scheduled-dnd .dnd-help { opacity: .8; font-size: 12px; line-height: 1.6; }
.scheduled-dnd .dnd-error { color: var(--error-text, #b30024); padding: 10px; border: 1px solid currentColor; border-radius: 4px; }
.scheduled-dnd .dnd-notice { padding: 12px; border: 1px solid var(--button-bg, #166de0); border-radius: 4px; }
.scheduled-dnd footer { display: flex; flex-wrap: wrap; gap: 8px; }
.scheduled-dnd .btn { border-radius: 4px; padding: 10px 16px; font-weight: 600; cursor: pointer; }
.scheduled-dnd .btn-primary { background: var(--button-bg, #166de0); color: var(--button-color, #fff); border: 1px solid transparent; }
.scheduled-dnd .btn-tertiary { background: transparent; color: var(--link-color, #166de0); border: 1px solid currentColor; }
.scheduled-dnd button:disabled { opacity: .5; cursor: default; }
.scheduled-dnd :focus-visible { outline: 2px solid var(--button-bg, #166de0); outline-offset: 3px; }
`;
