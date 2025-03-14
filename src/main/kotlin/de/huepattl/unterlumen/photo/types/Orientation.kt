package de.huepattl.unterlumen.photo.types

enum class Orientation(private val displayName: String) {
    HORIZONTAL("horizontal"),
    VERTICAL("vertical");

    override fun toString() = displayName

    companion object {

        fun of(str: String): Orientation = when (str.lowercase()) {
            "horizontal", "landscape" -> HORIZONTAL
            "vertical", "portrait" -> VERTICAL
            else -> throw IllegalArgumentException("unknown orientation: $str")
        }

    }
}