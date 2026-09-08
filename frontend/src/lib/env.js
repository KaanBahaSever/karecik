// Build-time settings, read from Vite's import.meta.env.
//
// They live in their own module rather than beside the code that uses them so a
// page can read one without importing App.jsx — which imports every page back,
// and made Landing.jsx and App.jsx import each other in a cycle.
//
// All of these are inlined by Vite at BUILD time, not read at runtime: changing
// one means rebuilding the bundle, not restarting anything.

/** Which tenant the landing page previews inside the phone mockup.
 *  Empty means no sample is deployed, and the mockup shows a static placeholder
 *  instead of an iframe pointing at a menu that does not exist. */
export const DEMO_BUSINESS_SLUG = import.meta.env.VITE_DEMO_BUSINESS || ''
