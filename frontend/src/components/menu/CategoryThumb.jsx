import { categoryEmoji } from '../../lib/category'
import { useImageFallback } from '../../lib/useImageFallback'

/**
 * The visual at the top of a category card in the customer menu — when the
 * category has one.
 *
 * A picture is optional, an emoji is optional, and so is the visual itself:
 *
 *   image    when there is a URL and the browser actually loaded it
 *   emoji    when there is no usable image and the icon is a real glyph
 *   nothing  otherwise — the card is then just the category's name
 *
 * There is deliberately no placeholder. An owner who picked neither a picture
 * nor an emoji asked for a text-only category, and drawing something in its
 * place — a plate glyph, an empty box — puts on their menu a thing they never
 * chose. What this component does guard against is a visual that is present
 * but unusable: an image whose file is gone falls back to the emoji, or to
 * nothing, never to a broken box; and a legacy icon code such as "coffee" is
 * not an emoji, so it is never printed as giant text.
 *
 * It MUST stay a module-level component. The load-failure state lives here, and
 * MenuContent declares fragments inside its own render body — anything declared
 * there is remounted on every render and would forget that the image failed.
 *
 * @param {string|null} imageUrl  - Category image URL; blank, missing or broken is fine
 * @param {string|null} emoji     - Category icon; anything that is not a glyph is ignored
 * @param {string}      className - Sizing classes, i.e. the card's visual height
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

  const glyph = categoryEmoji(emoji)
  if (!glyph) return null

  return (
    <div
      className={`flex w-full items-center justify-center overflow-hidden text-4xl ${className}`.trim()}
      aria-hidden="true"
    >
      {glyph}
    </div>
  )
}
