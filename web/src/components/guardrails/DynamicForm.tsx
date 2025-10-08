import { JSONSchema, JSONSchemaProperty } from '@/types/discovery'
import { Label } from '@/components/ui/label'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { Switch } from '@/components/ui/switch'
import { Slider } from '@/components/ui/slider'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Checkbox } from '@/components/ui/checkbox'
import { Badge } from '@/components/ui/badge'
import { Info } from 'lucide-react'
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from '@/components/ui/tooltip'

interface DynamicFormProps {
  schema: JSONSchema
  values: Record<string, any>
  onChange: (values: Record<string, any>) => void
  errors?: Record<string, string>
  disabled?: boolean
}

export function DynamicForm({
  schema,
  values,
  onChange,
  errors = {},
  disabled = false,
}: DynamicFormProps) {
  const handleFieldChange = (field: string, value: any) => {
    onChange({
      ...values,
      [field]: value,
    })
  }

  return (
    <div className="space-y-6">
      {Object.entries(schema.properties).map(([field, property]) => (
        <DynamicField
          key={field}
          field={field}
          property={property}
          value={values[field]}
          onChange={(value) => handleFieldChange(field, value)}
          error={errors[field]}
          required={schema.required?.includes(field)}
          disabled={disabled || property['ui:disabled'] || property['ui:readonly']}
        />
      ))}
    </div>
  )
}

interface DynamicFieldProps {
  field: string
  property: JSONSchemaProperty
  value: any
  onChange: (value: any) => void
  error?: string
  required?: boolean
  disabled?: boolean
}

function DynamicField({
  field,
  property,
  value,
  onChange,
  error,
  required,
  disabled,
}: DynamicFieldProps) {
  const title = property.title || field
  const description = property.description || property['ui:help']
  const widget = property['ui:widget']

  // Get the primary type
  const type = Array.isArray(property.type) ? property.type[0] : property.type

  return (
    <div className="space-y-2">
      <div className="flex items-center gap-2">
        <Label htmlFor={field} className="flex items-center gap-1">
          {title}
          {required && <span className="text-destructive">*</span>}
        </Label>
        {description && (
          <TooltipProvider>
            <Tooltip>
              <TooltipTrigger asChild>
                <Info className="h-4 w-4 text-muted-foreground cursor-help" />
              </TooltipTrigger>
              <TooltipContent>
                <p className="max-w-xs">{description}</p>
              </TooltipContent>
            </Tooltip>
          </TooltipProvider>
        )}
      </div>

      {renderInputField(field, property, type, widget, value, onChange, disabled)}

      {error && <p className="text-sm text-destructive">{error}</p>}
    </div>
  )
}

function renderInputField(
  field: string,
  property: JSONSchemaProperty,
  type: string,
  widget: string | undefined,
  value: any,
  onChange: (value: any) => void,
  disabled?: boolean
) {
  const placeholder = property['ui:placeholder']

  // Boolean type
  if (type === 'boolean') {
    return (
      <div className="flex items-center space-x-2">
        <Switch
          id={field}
          checked={value || false}
          onCheckedChange={onChange}
          disabled={disabled}
        />
        <Label htmlFor={field} className="text-sm text-muted-foreground">
          {value ? 'Enabled' : 'Disabled'}
        </Label>
      </div>
    )
  }

  // Number/Integer with slider widget or min/max
  if (
    (type === 'number' || type === 'integer') &&
    (widget === 'slider' || (property.minimum !== undefined && property.maximum !== undefined))
  ) {
    const min = property.minimum ?? 0
    const max = property.maximum ?? 100
    const step = type === 'integer' ? 1 : 0.01

    return (
      <div className="space-y-2">
        <div className="flex items-center justify-between">
          <span className="text-sm text-muted-foreground">{min}</span>
          <span className="text-sm font-medium">{value ?? property.default ?? min}</span>
          <span className="text-sm text-muted-foreground">{max}</span>
        </div>
        <Slider
          id={field}
          min={min}
          max={max}
          step={step}
          value={[value ?? property.default ?? min]}
          onValueChange={(vals) => onChange(vals[0])}
          disabled={disabled}
        />
      </div>
    )
  }

  // String with enum (dropdown)
  if (type === 'string' && property.enum) {
    return (
      <Select value={value || ''} onValueChange={onChange} disabled={disabled}>
        <SelectTrigger id={field}>
          <SelectValue placeholder={placeholder || 'Select an option'} />
        </SelectTrigger>
        <SelectContent>
          {property.enum.map((option) => (
            <SelectItem key={String(option)} value={String(option)}>
              {String(option)}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    )
  }

  // Array with enum items (multi-select checkboxes)
  if (type === 'array' && property.items?.enum) {
    const options = property.items.enum
    const selectedValues = value || []

    return (
      <div className="space-y-2">
        {options.map((option) => {
          const optionStr = String(option)
          const isChecked = selectedValues.includes(option)

          return (
            <div key={optionStr} className="flex items-center space-x-2">
              <Checkbox
                id={`${field}-${optionStr}`}
                checked={isChecked}
                onCheckedChange={(checked) => {
                  if (checked) {
                    onChange([...selectedValues, option])
                  } else {
                    onChange(selectedValues.filter((v: any) => v !== option))
                  }
                }}
                disabled={disabled}
              />
              <Label
                htmlFor={`${field}-${optionStr}`}
                className="text-sm font-normal cursor-pointer"
              >
                {optionStr}
              </Label>
            </div>
          )
        })}
      </div>
    )
  }

  // Array with string items (tags input)
  if (type === 'array' && property.items?.type === 'string' && !property.items.enum) {
    const tags = value || []

    return (
      <div className="space-y-2">
        <div className="flex flex-wrap gap-2">
          {tags.map((tag: string, index: number) => (
            <Badge key={index} variant="secondary" className="gap-1">
              {tag}
              <button
                type="button"
                onClick={() => onChange(tags.filter((_: string, i: number) => i !== index))}
                disabled={disabled}
                className="ml-1 hover:text-destructive"
              >
                ×
              </button>
            </Badge>
          ))}
        </div>
        <Input
          id={field}
          placeholder={placeholder || 'Type and press Enter to add'}
          onKeyDown={(e) => {
            if (e.key === 'Enter') {
              e.preventDefault()
              const input = e.currentTarget
              const newTag = input.value.trim()
              if (newTag && !tags.includes(newTag)) {
                onChange([...tags, newTag])
                input.value = ''
              }
            }
          }}
          disabled={disabled}
        />
      </div>
    )
  }

  // String with textarea widget or long format
  if (type === 'string' && (widget === 'textarea' || property.format === 'text')) {
    return (
      <Textarea
        id={field}
        value={value || ''}
        onChange={(e) => onChange(e.target.value)}
        placeholder={placeholder}
        disabled={disabled}
        rows={4}
      />
    )
  }

  // Number/Integer (simple input)
  if (type === 'number' || type === 'integer') {
    return (
      <Input
        id={field}
        type="number"
        value={value ?? ''}
        onChange={(e) => {
          const val = e.target.value
          onChange(val === '' ? undefined : type === 'integer' ? parseInt(val) : parseFloat(val))
        }}
        placeholder={placeholder}
        min={property.minimum}
        max={property.maximum}
        step={type === 'integer' ? 1 : 0.01}
        disabled={disabled}
      />
    )
  }

  // String (default input)
  if (type === 'string') {
    return (
      <Input
        id={field}
        type={property.format === 'password' ? 'password' : 'text'}
        value={value || ''}
        onChange={(e) => onChange(e.target.value)}
        placeholder={placeholder}
        minLength={property.minLength}
        maxLength={property.maxLength}
        pattern={property.pattern}
        disabled={disabled}
      />
    )
  }

  // Object type (nested form - recursive)
  if (type === 'object' && property.properties) {
    const nestedSchema: JSONSchema = {
      type: 'object',
      properties: property.properties,
      required: property.required,
    }

    return (
      <div className="pl-4 border-l-2 border-border">
        <DynamicForm
          schema={nestedSchema}
          values={value || {}}
          onChange={onChange}
          disabled={disabled}
        />
      </div>
    )
  }

  // Fallback: JSON input
  return (
    <Textarea
      id={field}
      value={typeof value === 'string' ? value : JSON.stringify(value, null, 2)}
      onChange={(e) => {
        try {
          onChange(JSON.parse(e.target.value))
        } catch {
          onChange(e.target.value)
        }
      }}
      placeholder={placeholder || 'Enter JSON value'}
      disabled={disabled}
      rows={4}
      className="font-mono text-sm"
    />
  )
}
