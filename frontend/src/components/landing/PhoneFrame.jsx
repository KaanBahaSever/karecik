/**
 * iPhone mockup drawn entirely with CSS.
 *
 * The content fills the screen area and anything overflowing is clipped.
 * Per the landing page rule there is no animation here, only a static shadow.
 *
 * @param {node}   children  - Content placed inside the screen
 * @param {string} className - Extra classes for the outer wrapper
 */
export default function PhoneFrame({ children, className = '' }) {
  return (
    <div className={`relative mx-auto w-full max-w-[340px] ${className}`}>
      {/* left edge: mute switch and volume buttons */}
      <div
        className="absolute left-[-3px] top-[16%] h-8 w-[3px] rounded-l bg-gray-800"
        aria-hidden="true"
      />
      <div
        className="absolute left-[-3px] top-[26%] h-12 w-[3px] rounded-l bg-gray-800"
        aria-hidden="true"
      />
      <div
        className="absolute left-[-3px] top-[38%] h-12 w-[3px] rounded-l bg-gray-800"
        aria-hidden="true"
      />

      {/* right edge: power button */}
      <div
        className="absolute right-[-3px] top-[30%] h-16 w-[3px] rounded-r bg-gray-800"
        aria-hidden="true"
      />

      {/* outer body */}
      <div className="relative aspect-[390/844] w-full rounded-[3rem] bg-gray-900 p-3 shadow-2xl">
        {/* screen area */}
        <div className="relative h-full w-full overflow-hidden rounded-[2.5rem] bg-white">
          {/*
            Safe area: the Dynamic Island sits at top-2 (8px) and is 26px tall,
            so its bottom edge is at 34px. Starting the content 44px down keeps
            it clear of the cutout — the same inset iOS itself uses.
          */}
          <div className="h-full w-full pt-[44px]">{children}</div>

          {/* dynamic island */}
          <div
            className="pointer-events-none absolute left-1/2 top-2 z-10 h-[26px] w-[92px] -translate-x-1/2 rounded-full bg-black"
            aria-hidden="true"
          />
        </div>
      </div>
    </div>
  )
}
