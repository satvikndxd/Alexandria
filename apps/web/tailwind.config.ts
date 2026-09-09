import type { Config } from "tailwindcss";

/**
 * Alexandria design tokens — "Illuminated Manuscript / Botanical Woodcut",
 * tuned against the reference sheet: warm parchment ground, near-monochrome
 * ink, sepia engravings, vermilion/gold held in reserve.
 *
 * Rules encoded here, enforced everywhere:
 *  - sharp editorial corners: the only radii are 0 and 2px ("nick")
 *  - no soft drop shadows: depth comes from hard offset "print-block" shadows
 *  - flat ink and flat color; texture comes from the paper grain overlay
 */
const config: Config = {
  content: ["./src/**/*.{ts,tsx}"],
  theme: {
    colors: {
      transparent: "transparent",
      current: "currentColor",
      parchment: {
        DEFAULT: "#E8DDC4",
        light: "#F0E7D2",
        page: "#ECE5D3",
        panel: "#F0EAD9",
        dark: "#D9CCAE",
        deep: "#C9B992",
      },
      ink: {
        DEFAULT: "#171A14",
        soft: "#3C4036",
        faint: "#6B6F62",
      },
      sepia: {
        DEFAULT: "#5F584A",
        light: "#8A8172",
      },
      botanical: {
        DEFAULT: "#0D3B2E",
        mid: "#124A38",
        light: "#1B5B42",
      },
      vermilion: {
        DEFAULT: "#D9471F",
        bright: "#E84B1C",
      },
      gold: {
        DEFAULT: "#C79522",
        bright: "#D4A62A",
      },
    },
    fontFamily: {
      display: ["var(--font-blackletter)", "serif"],
      body: ["var(--font-garamond)", "Georgia", "serif"],
      smallcaps: ["var(--font-garamond)", "Georgia", "serif"],
    },
    extend: {
      borderRadius: {
        none: "0",
        nick: "2px",
      },
      boxShadow: {
        // hard print-block offsets — never blurred
        block: "4px 4px 0 0 #171A14",
        "block-sm": "2px 2px 0 0 #171A14",
        "block-green": "4px 4px 0 0 #0D3B2E",
        "block-vermilion": "4px 4px 0 0 #D9471F",
        none: "none",
      },
      letterSpacing: {
        engraved: "0.22em",
        wordmark: "0.34em",
      },
      maxWidth: {
        folio: "76rem",
      },
    },
  },
  plugins: [],
};

export default config;
