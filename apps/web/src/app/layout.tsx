import type { Metadata } from "next";
import { EB_Garamond, UnifrakturMaguntia } from "next/font/google";
import "./globals.css";

/**
 * Type system, per the design brief:
 *  - EB Garamond carries everything — the classical literary body face, and
 *    (set in tracked capitals) the masthead and section titles, exactly as the
 *    reference sheet does.
 *  - UnifrakturMaguntia is reserved for illuminated initials and drop caps.
 *    It never sets running text: blackletter at body sizes is unreadable, and
 *    the reference keeps it ornamental.
 */
const garamond = EB_Garamond({
  subsets: ["latin", "cyrillic", "greek"],
  style: ["normal", "italic"],
  variable: "--font-garamond",
  display: "swap",
});

const blackletter = UnifrakturMaguntia({
  weight: "400",
  subsets: ["latin"],
  variable: "--font-blackletter",
  display: "swap",
});

export const metadata: Metadata = {
  title: {
    default: "Alexandria — A Human Library",
    template: "%s · Alexandria",
  },
  description:
    "An open-source, ad-free social reading platform. Books written by humans, discussed by humans, explained by humans, read by humans.",
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en" className={`${garamond.variable} ${blackletter.variable}`}>
      <body>{children}</body>
    </html>
  );
}
