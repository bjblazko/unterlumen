package de.huepattl.unterlumen.photo.types

data class Quality(
    val colourDepth: ColourDepth? = null,
)

class ColourDepth(value: Int, unit: Unit) : Measurement<Int, ColourDepth.Unit>(value, unit) {

    enum class Unit(private val displayName: String) {
        BIT("bit");

        override fun toString() = displayName
    }

    companion object {

        fun of(value: Int): ColourDepth = ColourDepth(value, Unit.BIT)

        fun of(str: String): ColourDepth {
            val parts = str.split(" ")
            require(parts.size == 2) { "invalid format, expected '<value> <unit>'" }

            val value = parts[0].toInt()
            val unit = when (parts[1].lowercase()) {
                "bit", "bits" -> Unit.BIT
                else -> throw IllegalArgumentException("unknown unit: ${parts[1]}")
            }

            return ColourDepth(value, unit)
        }
    }

}
