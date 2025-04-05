package de.huepattl.unterlumen.photo.types

class Dimensions(
    val height: Length? = null,
    val width: Length? = null,
) {
    companion object {

        fun of(height: Int, width: Int): Dimensions =
            Dimensions(
                width = Length(width, Length.Unit.PIXELS),
                height = Length(height, Length.Unit.PIXELS)
            )

    }
}

class Length(value: Int, unit: Unit) : Measurement<Int, Length.Unit>(value, unit) {

    enum class Unit(private val displayName: String) {
        PIXELS("px"),
        CENTIMETERS("cm"),
        MILLIMETERS("mm"),
        METERS("m"),
        KILOMETERS("km");

        override fun toString() = displayName
    }

    fun normalise(): Length {
        val normalised = when (this.unit) {
            Unit.KILOMETERS -> value * 1000
            else -> value
        }
        return Length(value = normalised, unit = Unit.METERS)
    }

    companion object {

        fun of(value: Int): Length = Length(value, Unit.PIXELS)

        fun of(str: String): Length {
            val parts = str.split(" ")
            require(parts.size == 2) { "invalid format, expected '<value> <unit>'" }

            val value = parts[0].replace(",", ".").toDouble().toInt().toInt() // e.g. iPhone has '4.2 mm'
            val unit = when (parts[1].lowercase()) {
                "px", "pixel", "pixels" -> Unit.PIXELS
                "mm", "millimeter", "millimetre","millimeters", "millimetres" -> Unit.MILLIMETERS
                "cm", "centimeter", "centimetre", "centimeters", "centimetres" -> Unit.CENTIMETERS
                "m", "meter", "metre", "meters", "metres" -> Unit.METERS
                else -> throw IllegalArgumentException("unknown unit: ${parts[1]}")
            }

            return Length(value, unit)
        }
    }

}

