package de.huepattl.unterlumen.photo.types

class FileSize(value: Long, unit: Unit) : Measurement<Long, FileSize.Unit>(value, unit) {

    enum class Unit(private val displayName: String) {
        BYTE("byte");

        override fun toString() = displayName
    }

    companion object {

        fun of(value: Long): FileSize = FileSize(value, Unit.BYTE)

        fun of(str: String): FileSize {
            val parts = str.split(" ")
            require(parts.size == 2) { "invalid format, expected '<value> <unit>'" }

            val value = parts[0].toLong()
            val unit = when (parts[1].lowercase()) {
                "byte", "bytes", "b", "B" -> Unit.BYTE
                else -> throw IllegalArgumentException("unknown unit: ${parts[1]}")
            }

            return FileSize(value, unit)
        }
    }

}
