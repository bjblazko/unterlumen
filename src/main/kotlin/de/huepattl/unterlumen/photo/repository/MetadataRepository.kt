package de.huepattl.unterlumen.photo.repository

import de.huepattl.unterlumen.photo.Metadata
import de.huepattl.unterlumen.photo.types.Aperture
import de.huepattl.unterlumen.photo.types.ColourDepth
import de.huepattl.unterlumen.photo.types.Dimensions
import de.huepattl.unterlumen.photo.types.Exposure
import de.huepattl.unterlumen.photo.types.ExposureTime
import de.huepattl.unterlumen.photo.types.FileSize
import de.huepattl.unterlumen.photo.types.GeoCoordinate
import de.huepattl.unterlumen.photo.types.GeoLocation
import de.huepattl.unterlumen.photo.types.Length
import de.huepattl.unterlumen.photo.types.Lens
import de.huepattl.unterlumen.photo.types.Orientation
import de.huepattl.unterlumen.photo.types.Quality
import io.quarkus.hibernate.orm.panache.kotlin.PanacheRepositoryBase
import jakarta.enterprise.context.ApplicationScoped
import jakarta.persistence.*
import java.math.BigDecimal
import java.time.LocalDateTime
import java.util.*


@Entity
@Table(name = "metadata")
class MetadataEntity() {

    @Id
    @Column(name = "id", nullable = false, unique = true)
    var id: String? = UUID.randomUUID().toString()

    @Column(name = "title", nullable = true)
    var title: String? = null

    @Column(name = "filename", nullable = true)
    var filename: String? = null

    @Column(name = "mime_type", nullable = true)
    var mimeType: String? = null

    @Column(name = "description", nullable = true)
    var description: String? = null

    @Column(name = "tags", nullable = true)
    var tags: String? = null

    @Column(name = "width", nullable = true)
    var width: Int? = 0

    @Column(name = "height", nullable = true)
    var height: Int? = 0

    @OneToOne(cascade = [(CascadeType.ALL)], orphanRemoval = true)
    @JoinColumn(name = "exposure_id", nullable = true)
    var exposure: ExposureEntity? = null

    @OneToOne(cascade = [(CascadeType.ALL)], orphanRemoval = true)
    @JoinColumn(name = "lens_settings_id", nullable = true)
    var lensSettingsId: LensSettingsEntity? = null

    @Column(name = "software", nullable = true)
    var software: String? = null

    @Column(name = "artist", nullable = true)
    var artist: String? = null

    @Column(name = "copyright", nullable = true)
    var copyright: String? = null

    @Column(name = "camera_brand", nullable = true)
    var cameraBrand: String? = null

    @Column(name = "camera_model", nullable = true)
    var cameraModel: String? = null

    @Column(name = "orientation", nullable = true)
    var orientation: String? = null

    @Column(name = "whitebalance_mode", nullable = true)
    var whiteBalanceMode: String? = null

    @Column(name = "look", nullable = true)
    var look: String? = null

    @Column(name = "quality", nullable = true)
    var quality: Int? = null

    @Column(name = "quality_unit", nullable = true)
    var qualityUnit: String? = null

    @Column(name = "size_in_bytes", nullable = true)
    var sizeInBytes: Long? = null

    @OneToOne(cascade = [(CascadeType.ALL)], orphanRemoval = true)
    @JoinColumn(name = "geolocation_id", nullable = true)
    var geolocation: GeolocationEntity? = null

    @Column(name = "photo_created_at", nullable = true)
    var photoCreatedAt: LocalDateTime? = null

    fun toDomainObject(): Metadata {
        return Metadata(
            id = UUID.fromString(this.id),
            title = this.title,
            description = this.description,
            filename = this.filename,
            fileSize = FileSize.of(this.sizeInBytes ?: 0),
            mimeType = this.mimeType,
            tags = this.tags?.split(",") ?: listOf(),
            software = this.software,
            artist = this.artist,
            copyright = this.copyright,
            cameraBrand = this.cameraBrand,
            cameraModel = this.cameraModel,
            whiteBalanceMode = this.whiteBalanceMode,
            look = this.look,
            quality = Quality(colourDepth = ColourDepth(value = this.quality ?: 8, unit = ColourDepth.Unit.BIT)),
            orientation = Orientation.of(this.orientation ?: "horizontal"),
            dimensions = Dimensions.of(width = this.width ?: 0, height = this.height ?: 0),
            exposure = Exposure(
                iso = this.exposure?.iso,
                meteringMode = this.exposure?.meteringMode,
                mode = this.exposure?.exposureMode,
                time = ExposureTime.of(
                    numerator = this.exposure?.timeNumerator ?: 0,
                    denominator = this.exposure?.timeDenominator ?: 1,
                    unit = ExposureTime.Unit.SECONDS
                ),
                compensation = this.exposure?.compensation,
            ),
            lens = Lens(
                brand = this.lensSettingsId?.brand,
                model = this.lensSettingsId?.model,
                focalLength = Length(value = this.lensSettingsId?.focalLength ?: 0, unit = Length.Unit.MILLIMETERS),
                aperture = Aperture(this.lensSettingsId?.aperture ?: BigDecimal.ZERO),
            ),
            createdAt = this.photoCreatedAt,
            geoLocation = GeoLocation(
                latitude = GeoCoordinate(
                    degrees = this.geolocation?.latitudeDegrees ?: 0,
                    minutes = this.geolocation?.latitudeMinutes ?: 0,
                    seconds = this.geolocation?.latitudeSeconds ?: BigDecimal.ZERO
                ),
                longitude = GeoCoordinate(
                    degrees = this.geolocation?.longitudeDegrees ?: 0,
                    minutes = this.geolocation?.longitudeMinutes ?: 0,
                    seconds = this.geolocation?.longitudeSeconds ?: BigDecimal("0.0")
                ),
                altitude = Length(this.geolocation?.altitude ?: 0, Length.Unit.METERS)
            )
        )
    }

    companion object {

        fun of(metadata: Metadata): MetadataEntity {
            var entity = MetadataEntity()
            entity.apply {
                id = metadata.id.toString()
                this.title = metadata.title
                description = metadata.description
                filename = metadata.filename
                sizeInBytes = metadata.fileSize?.value
                mimeType = metadata.mimeType
                software = metadata.software
                artist = metadata.artist
                copyright = metadata.copyright
                cameraBrand = metadata.cameraBrand
                cameraModel = metadata.cameraModel
                orientation = metadata.orientation?.name
                whiteBalanceMode = metadata.whiteBalanceMode
                look = metadata.look
                quality = metadata.quality?.colourDepth?.value
                qualityUnit = metadata.quality?.colourDepth?.unit?.name
                tags = metadata.tags.joinToString(",")
                width = metadata.dimensions?.width?.value ?: 0
                height = metadata.dimensions?.height?.value ?: 0
                software = metadata.software
                artist = metadata.artist
                copyright = metadata.copyright
                exposure = ExposureEntity().apply {
                    iso = metadata.exposure?.iso
                    exposureMode = metadata.exposure?.mode
                    meteringMode = metadata.exposure?.meteringMode
                    timeNumerator = metadata.exposure?.time?.value?.numerator
                    timeDenominator = metadata.exposure?.time?.value?.denominator
                    compensation = metadata.exposure?.compensation
                }
                lensSettingsId = LensSettingsEntity().apply {
                    brand = metadata.lens?.brand
                    model = metadata.lens?.model
                    aperture = metadata.lens?.aperture?.value
                    focalLength = metadata.lens?.focalLength?.value
                }
                photoCreatedAt = metadata.createdAt
            }
            if (metadata.geoLocation != null) {
                entity.geolocation = GeolocationEntity().apply {
                    latitudeDegrees = metadata.geoLocation.latitude.degrees
                    latitudeMinutes = metadata.geoLocation.latitude.minutes
                    latitudeSeconds = metadata.geoLocation.latitude.seconds
                    longitudeDegrees = metadata.geoLocation.longitude.degrees
                    longitudeMinutes = metadata.geoLocation.longitude.minutes
                    longitudeSeconds = metadata.geoLocation.longitude.seconds
                    altitude = metadata.geoLocation.altitude.normalise().value
                }
            }

            return entity
        }
    }
}


@Entity
@Table(name = "exposure")
class ExposureEntity() {
    @Id
    @Column(name = "id", nullable = false, unique = true)
    var id: String = UUID.randomUUID().toString()

    @Column(name = "iso", nullable = true)
    var iso: Int? = null

    @Column(name = "mode", nullable = true)
    var exposureMode: String? = null

    @Column(name = "metering", nullable = true)
    var meteringMode: String? = null

    @Column(name = "time_numerator", nullable = true)
    var timeNumerator: Int? = null

    @Column(name = "time_denominator", nullable = true)
    var timeDenominator: Int? = null

    @Column(name = "compensation", nullable = true)
    var compensation: String? = null

}

@Entity
@Table(name = "lens")
class LensSettingsEntity() {

    @Id
    @Column(name = "id", nullable = false, unique = true)
    var id: String? = UUID.randomUUID().toString()

    @Column(name = "brand", nullable = true)
    var brand: String? = null

    @Column(name = "model", nullable = true)
    var model: String? = null

    @Column(name = "aperture", nullable = true)
    var aperture: BigDecimal? = null

    @Column(name = "focal_length", nullable = true)
    var focalLength: Int? = null
}

@Entity
@Table(name = "geolocation")
class GeolocationEntity() {

    @Id
    @Column(name = "id", nullable = false, unique = true)
    var id: String? = UUID.randomUUID().toString()

    @Column(name = "lat_deg", nullable = true)
    var latitudeDegrees: Int? = null

    @Column(name = "lat_min", nullable = true)
    var latitudeMinutes: Int? = null

    @Column(name = "lat_sec", nullable = true)
    var latitudeSeconds: BigDecimal? = null

    @Column(name = "long_deg", nullable = true)
    var longitudeDegrees: Int? = null

    @Column(name = "long_min", nullable = true)
    var longitudeMinutes: Int? = null

    @Column(name = "long_sec", nullable = true)
    var longitudeSeconds: BigDecimal? = null

    @Column(name = "altitude", nullable = true)
    var altitude: Int? = null
}

@ApplicationScoped
class MetadataRepository : PanacheRepositoryBase<MetadataEntity, String> {

    fun persist(metadata: Metadata) {
        this.persist(MetadataEntity.of(metadata))
    }

}