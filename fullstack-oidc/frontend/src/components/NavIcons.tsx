import type { ReactNode, SVGProps } from 'react';

/**
 * Custom-crafted navigation icon set for the OIDC template shell.
 *
 * Hand-drawn rather than pulled from an icon library so the set is cohesive and
 * owned by the template: one 24×24 grid, a single 1.7px stroke, round caps and
 * joins, and `stroke="currentColor"` throughout — so each icon inherits the
 * nav-link colour and turns emerald on the active route with zero extra CSS.
 */

type IconProps = Omit<SVGProps<SVGSVGElement>, 'children'>;

function Glyph({ children, ...props }: IconProps & { children: ReactNode }) {
  return (
    <svg
      viewBox="0 0 24 24"
      width="20"
      height="20"
      fill="none"
      stroke="currentColor"
      strokeWidth={1.7}
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
      focusable="false"
      {...props}
    >
      {children}
    </svg>
  );
}

/** Overview — four rounded tiles reading as a dashboard / at-a-glance grid. */
export function OverviewIcon(props: IconProps) {
  return (
    <Glyph {...props}>
      <rect x="3.5" y="3.5" width="7" height="7" rx="2" />
      <rect x="13.5" y="3.5" width="7" height="7" rx="2" />
      <rect x="3.5" y="13.5" width="7" height="7" rx="2" />
      <rect x="13.5" y="13.5" width="7" height="7" rx="2" />
    </Glyph>
  );
}

/** My Identity — a clean person bust. */
export function IdentityIcon(props: IconProps) {
  return (
    <Glyph {...props}>
      <circle cx="12" cy="8.5" r="3.5" />
      <path d="M5.5 20a6.5 6.5 0 0 1 13 0" />
    </Glyph>
  );
}

/** Roles & Access — a shield with an approval check. */
export function RolesIcon(props: IconProps) {
  return (
    <Glyph {...props}>
      <path d="M12 3.2 18.6 6v5.2c0 4.3-2.8 7.3-6.6 8.6-3.8-1.3-6.6-4.3-6.6-8.6V6z" />
      <path d="M9.2 12.2 11 14l3.9-4.1" />
    </Glyph>
  );
}

/** My Entries — a document with a folded corner and a couple of text lines. */
export function EntriesIcon(props: IconProps) {
  return (
    <Glyph {...props}>
      <path d="M13 3H8a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h8a2 2 0 0 0 2-2V8z" />
      <path d="M13 3v5h5" />
      <path d="M9 13.5h6" />
      <path d="M9 16.5h4" />
    </Glyph>
  );
}

/** Shared Notes — three connected nodes (the universal "share" motif). */
export function SharedNotesIcon(props: IconProps) {
  return (
    <Glyph {...props}>
      <circle cx="6" cy="12" r="2.3" />
      <circle cx="17.5" cy="6" r="2.3" />
      <circle cx="17.5" cy="18" r="2.3" />
      <path d="M8 11 15.5 7.1" />
      <path d="M8 13l7.5 3.9" />
    </Glyph>
  );
}

/** Silent SSO — a lock flanked by two arcs, evoking a quiet background refresh. */
export function SilentSSOIcon(props: IconProps) {
  return (
    <Glyph {...props}>
      <rect x="8.5" y="11" width="7" height="6" rx="1.6" />
      <path d="M10 11V9.8a2 2 0 0 1 4 0V11" />
      <path d="M12 13.4v1.3" />
      <path d="M5.6 9.4a6 6 0 0 0 0 5.2" />
      <path d="M18.4 9.4a6 6 0 0 1 0 5.2" />
    </Glyph>
  );
}

/** How It Works — an open book / docs spread. */
export function HowItWorksIcon(props: IconProps) {
  return (
    <Glyph {...props}>
      <path d="M12 6.6C10 5.3 7.5 5.3 5 6.6v11c2.5-1.3 5-1.3 7 0" />
      <path d="M12 6.6c2-1.3 4.5-1.3 7 0v11c-2.5-1.3-5-1.3-7 0" />
      <path d="M12 6.6v11" />
    </Glyph>
  );
}
