package de.huepattl.unterlumen.photo.types

class Dimensions(
    val height: Length? = null,
    val width: Length? = null,
) {
    companion object {

        fun of(height: Int, width: Int): Dimensions =
            Dimensions(
                Length(height, Length.Unit.PIXELS),
                Length(height, Length.Unit.PIXELS)
            )

    }
}

class Length(value: Int, unit: Unit) : Measurement<Int, Length.Unit>(value, unit) {

    enum class Unit(private val displayName: String) {
        PIXELS("px"),
        CENTIMETERS("cm"),
        METERS("m");

        override fun toString() = displayName
    }

    companion object {

        fun of(value: Int): Length = Length(value, Unit.PIXELS)

        fun of(str: String): Length {
            val parts = str.split(" ")
            require(parts.size == 2) { "Invalid format, expected '<value> <unit>'" }

            val value = parts[0].toInt()
            val unit = when (parts[1]) {
                "px", "Pixel", "Pixels", "pixel", "pixels" -> Unit.PIXELS
                "cm", "centimeter", "centimeters" -> Unit.CENTIMETERS
                "m", "meter", "meters" -> Unit.METERS
                else -> throw IllegalArgumentException("Unknown unit: ${parts[1]}")
            }

            return Length(value, unit)
        }
    }

}

