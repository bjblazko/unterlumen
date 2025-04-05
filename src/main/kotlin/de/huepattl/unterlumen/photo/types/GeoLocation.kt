package de.huepattl.unterlumen.photo.types

import java.math.BigDecimal

data class GeoLocation(
    val latitude: GeoCoordinate,
    val longitude: GeoCoordinate,
    val altitude: Length,
)

data class GeoCoordinate(
    val degrees: Int,
    val minutes: Int,
    val seconds: BigDecimal,
) {

    init {
        require(degrees in -90..90) { "degrees must be between -90 and 90" }
        require(minutes in 0..59) { "minutes must be between 0 and 59" }
        require(seconds >= BigDecimal.ZERO && seconds < BigDecimal(60)) { "seconds must be between 0 and 59.9999" }
    }

    companion object {

        fun parse(input: String): GeoCoordinate {
            val regex = """(-?\d+)°\s+(\d+)'?\s+([\d.,]+)"?""".toRegex()
            val matchResult = regex.matchEntire(input.trim()) ?: throw IllegalArgumentException("invalid format: $input")

            val (deg, min, sec) = matchResult.destructured
            return GeoCoordinate(
                degrees = deg.toInt(),
                minutes = min.toInt(),
                seconds = sec.replace(',', '.').toBigDecimal()
            )
        }

    }

}