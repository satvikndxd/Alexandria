import type { ReactNode } from "react";
import { SideRail, MobileBar, Masthead, RightRail, type RightRailData } from "./Rail";

/**
 * The folio frame: an outer double rule enclosing three columns — rail,
 * content, context — exactly as the reference sheet composes the page.
 * On small screens the rail collapses to a bottom bar and the context rail
 * yields its space to the reading column.
 */
export function PageFrame({
  pathname,
  epigraph,
  attribution,
  rightRail,
  children,
}: {
  pathname: string;
  epigraph: string;
  attribution: string;
  rightRail?: RightRailData;
  children: ReactNode;
}) {
  return (
    <div className="min-h-screen px-2 py-2 sm:px-4 sm:py-3">
      <div className="sheet-frame mx-auto max-w-folio">
        <div className="grid grid-cols-1 lg:grid-cols-[172px_minmax(0,1fr)] xl:grid-cols-[172px_minmax(0,1fr)_268px]">
          <div className="hidden lg:block">
            <SideRail pathname={pathname} />
          </div>
          <main className="min-w-0 pb-24 lg:pb-8">
            <Masthead epigraph={epigraph} attribution={attribution} />
            <div className="hairline mx-6 sm:mx-8" aria-hidden />
            <div className="px-6 pb-10 pt-6 sm:px-8">{children}</div>
          </main>
          {rightRail ? (
            <div className="hidden xl:block">
              <RightRail data={rightRail} />
            </div>
          ) : null}
        </div>
      </div>
      <MobileBar pathname={pathname} />
    </div>
  );
}
