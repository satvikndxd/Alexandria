import type { Metadata } from "next";
import { EB_Garamond, UnifrakturMaguntia } from "next/font/google";
import { Shell } from "@/components/Shell";
import "./globals.css";

const garamond = EB_Garamond({
  subsets: ["latin", "cyrillic"],
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
      <body>
        <Shell>{children}</Shell>
      </body>
    </html>
  );
}
