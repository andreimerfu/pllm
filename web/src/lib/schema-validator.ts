import { JSONSchema, JSONSchemaProperty } from '@/types/discovery'

export interface ValidationError {
  field: string
  message: string
  type: string
}

export class SchemaValidator {
  private schema: JSONSchema

  constructor(schema: JSONSchema) {
    this.schema = schema
  }

  /**
   * Validate a configuration object against the schema
   */
  validate(config: Record<string, any>): ValidationError[] {
    const errors: ValidationError[] = []

    // Check required fields
    if (this.schema.required) {
      for (const field of this.schema.required) {
        if (config[field] === undefined || config[field] === null) {
          errors.push({
            field,
            message: `${this.schema.properties[field]?.title || field} is required`,
            type: 'required',
          })
        }
      }
    }

    // Validate each field
    for (const [field, value] of Object.entries(config)) {
      const property = this.schema.properties[field]
      if (!property) continue

      const fieldErrors = this.validateProperty(field, value, property)
      errors.push(...fieldErrors)
    }

    return errors
  }

  /**
   * Validate a single property
   */
  private validateProperty(
    field: string,
    value: any,
    property: JSONSchemaProperty
  ): ValidationError[] {
    const errors: ValidationError[] = []
    const title = property.title || field

    // Handle null values
    if (value === null || value === undefined) {
      return errors
    }

    // Type validation
    const types = Array.isArray(property.type) ? property.type : [property.type]
    const actualType = this.getValueType(value)

    if (!types.includes(actualType as any) && !types.includes('null')) {
      errors.push({
        field,
        message: `${title} must be of type ${types.join(' or ')}`,
        type: 'type',
      })
      return errors // Don't continue validation if type is wrong
    }

    // String validations
    if (actualType === 'string') {
      if (property.minLength !== undefined && value.length < property.minLength) {
        errors.push({
          field,
          message: `${title} must be at least ${property.minLength} characters`,
          type: 'minLength',
        })
      }
      if (property.maxLength !== undefined && value.length > property.maxLength) {
        errors.push({
          field,
          message: `${title} must be at most ${property.maxLength} characters`,
          type: 'maxLength',
        })
      }
      if (property.pattern) {
        const regex = new RegExp(property.pattern)
        if (!regex.test(value)) {
          errors.push({
            field,
            message: `${title} does not match required pattern`,
            type: 'pattern',
          })
        }
      }
      if (property.enum && !property.enum.includes(value)) {
        errors.push({
          field,
          message: `${title} must be one of: ${property.enum.join(', ')}`,
          type: 'enum',
        })
      }
    }

    // Number validations
    if (actualType === 'number' || actualType === 'integer') {
      if (property.minimum !== undefined && value < property.minimum) {
        errors.push({
          field,
          message: `${title} must be at least ${property.minimum}`,
          type: 'minimum',
        })
      }
      if (property.maximum !== undefined && value > property.maximum) {
        errors.push({
          field,
          message: `${title} must be at most ${property.maximum}`,
          type: 'maximum',
        })
      }
    }

    // Array validations
    if (actualType === 'array' && property.items) {
      value.forEach((item: any, index: number) => {
        const itemErrors = this.validateProperty(
          `${field}[${index}]`,
          item,
          property.items!
        )
        errors.push(...itemErrors)
      })
    }

    // Object validations
    if (actualType === 'object' && property.properties) {
      for (const [subField, subValue] of Object.entries(value)) {
        const subProperty = property.properties[subField]
        if (subProperty) {
          const subErrors = this.validateProperty(
            `${field}.${subField}`,
            subValue,
            subProperty
          )
          errors.push(...subErrors)
        }
      }

      // Check required sub-fields
      if (property.required) {
        for (const requiredField of property.required) {
          if (value[requiredField] === undefined) {
            errors.push({
              field: `${field}.${requiredField}`,
              message: `${requiredField} is required`,
              type: 'required',
            })
          }
        }
      }
    }

    return errors
  }

  /**
   * Get the JSON Schema type of a value
   */
  private getValueType(value: any): string {
    if (value === null) return 'null'
    if (Array.isArray(value)) return 'array'

    const type = typeof value
    if (type === 'number') {
      return Number.isInteger(value) ? 'integer' : 'number'
    }

    return type
  }

  /**
   * Get default configuration from schema
   */
  static getDefaults(schema: JSONSchema): Record<string, any> {
    const defaults: Record<string, any> = {}

    for (const [field, property] of Object.entries(schema.properties)) {
      if (property.default !== undefined) {
        defaults[field] = property.default
      } else if (schema.required?.includes(field)) {
        // Provide sensible defaults for required fields
        defaults[field] = this.getDefaultForType(property)
      }
    }

    return defaults
  }

  /**
   * Get a sensible default value for a property type
   */
  private static getDefaultForType(property: JSONSchemaProperty): any {
    const type = Array.isArray(property.type) ? property.type[0] : property.type

    switch (type) {
      case 'string':
        return property.enum ? property.enum[0] : ''
      case 'number':
      case 'integer':
        return property.minimum ?? 0
      case 'boolean':
        return false
      case 'array':
        return []
      case 'object':
        return {}
      default:
        return null
    }
  }

  /**
   * Check if a configuration is valid
   */
  isValid(config: Record<string, any>): boolean {
    return this.validate(config).length === 0
  }

  /**
   * Get validation errors as a map by field
   */
  getErrorMap(config: Record<string, any>): Record<string, string> {
    const errors = this.validate(config)
    const errorMap: Record<string, string> = {}

    for (const error of errors) {
      if (!errorMap[error.field]) {
        errorMap[error.field] = error.message
      }
    }

    return errorMap
  }
}

/**
 * Utility function to validate configuration
 */
export function validateConfiguration(
  schema: JSONSchema,
  config: Record<string, any>
): { valid: boolean; errors: ValidationError[]; errorMap: Record<string, string> } {
  const validator = new SchemaValidator(schema)
  const errors = validator.validate(config)
  const errorMap = validator.getErrorMap(config)

  return {
    valid: errors.length === 0,
    errors,
    errorMap,
  }
}
