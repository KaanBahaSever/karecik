import { categoryEmoji } from '../../lib/category'
import { useImageFallback } from '../../lib/useImageFallback'

/** Drawn when a category has neither a usable image nor an emoji. */
const DEFAULT_CATEGORY_GLYPH = '🍽️'

/**
 * The visual at the top of a category card in the customer menu.
 *
 * Both inputs are optional and the fallback order is fixed:
 *
 *   image  when there is a URL and the browser actually loaded it
 *   emoji  when the image is missing or failed, and the icon is a real glyph
 *   🍽️     when neither is usable
 *
 * so a card never shows an empty or broken box, and a legacy icon code such as
 * "coffee" is never printed as giant text.
 *
 * It MUST stay a module-level component. The load-failure state lives here, and
 * MenuContent declares fragments inside its own render body — anything declared
 * there is remounted on every render and would forget that the image failed.
 *
 * @param {string|null} imageUrl  - Category image URL; blank, missing or broken is fine
 * @param {string|null} emoji     - Category icon; anything that is not a glyph is ignored
 * @param {string}      className - Sizing classes, i.e. the card's image height
 */
export default function CategoryThumb({ imageUrl, emoji, className = '' }) {
  const image = useImageFallback(imageUrl)

  if (image.src) {
    return (
      <img
        src={image.src}
        alt=""
        onError={image.onError}
        className={`w-full object-cover ${className}`.trim()}
      />
    )
  }

  return (
    <div
      className={`flex w-full items-center justify-center overflow-hidden text-4xl ${className}`.trim()}
      aria-hidden="true"
    >
      {categoryEmoji(emoji) || DEFAULT_CATEGORY_GLYPH}
    </div>
  )
}
